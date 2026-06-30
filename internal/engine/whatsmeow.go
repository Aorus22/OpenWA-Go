package engine

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	waCommon "go.mau.fi/whatsmeow/proto/waCommon"
	"google.golang.org/protobuf/proto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/skip2/go-qrcode"
	_ "github.com/mattn/go-sqlite3"
)

// WhatsmeowAdapter implements IWhatsAppEngine using the whatsmeow library.
type WhatsmeowAdapter struct {
	mu          sync.RWMutex
	client      *whatsmeow.Client
	status      EngineStatus
	qrCode      *string
	phoneNumber *string
	pushName    *string
	callbacks   EngineEventCallbacks

	config     WhatsmeowConfig
	sqlStore   *sqlstore.Container

	ctx        context.Context
	cancel     context.CancelFunc
	intentionalClose bool

	log zerolog.Logger
}

// WhatsmeowConfig holds configuration for the whatsmeow adapter.
type WhatsmeowConfig struct {
	SessionID       string
	AuthDir         string
	ProxyURL        string
	SyncFullHistory bool
	LogLevel        string
}

// NewWhatsmeowAdapter creates a new whatsmeow engine adapter.
func NewWhatsmeowAdapter(cfg WhatsmeowConfig) *WhatsmeowAdapter {
	ctx, cancel := context.WithCancel(context.Background())
	return &WhatsmeowAdapter{
		status: StatusDisconnected,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
		log: log.With().Str("session", cfg.SessionID).Str("engine", "whatsmeow").Logger(),
	}
}

// Initialize connects to WhatsApp and starts handling events.
func (w *WhatsmeowAdapter) Initialize(callbacks EngineEventCallbacks) error {
	w.mu.Lock()
	w.callbacks = callbacks
	w.intentionalClose = false
	w.mu.Unlock()

	w.setStatus(StatusInitializing)

	// Ensure auth directory exists
	dbPath := filepath.Join(w.config.AuthDir, fmt.Sprintf("%s.db", w.config.SessionID))
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		w.setStatus(StatusFailed)
		w.fireError(fmt.Sprintf("Failed to create auth dir: %v", err))
		return err
	}

	// Initialize SQL store - newer API requires context
	wLog := waLog.Stdout("whatsmeow", "warn", true)
	container, err := sqlstore.New(w.ctx, "sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbPath), wLog)
	if err != nil {
		w.setStatus(StatusFailed)
		w.fireError(fmt.Sprintf("Failed to create store: %v", err))
		return err
	}

	device, err := container.GetFirstDevice(w.ctx)
	if err != nil {
		w.setStatus(StatusFailed)
		w.fireError(fmt.Sprintf("Failed to get device: %v", err))
		return err
	}

	w.sqlStore = container

	// Create client
	client := whatsmeow.NewClient(device, wLog)
	w.client = client

	// Configure
	client.EnableAutoReconnect = true
	client.AutoReconnectErrors = 10

	// Register event handlers
	client.AddEventHandler(w.handleEvent)

	// Connect
	if err := client.Connect(); err != nil {
		w.setStatus(StatusFailed)
		w.fireError(fmt.Sprintf("Connect failed: %v", err))
		return err
	}

	// If already logged in, we'll get Connected event
	if device.ID != nil {
		w.log.Info().Msg("Device already has stored credentials, waiting for connection...")
		return nil
	}

	return nil
}

func (w *WhatsmeowAdapter) handleEvent(evt interface{}) {
	switch e := evt.(type) {
	case *events.QR:
		w.handleQR(e)
	case *events.Connected:
		w.handleConnected()
	case *events.PairSuccess:
		w.handlePairSuccess(e)
	case *events.Disconnected:
		w.handleDisconnected()
	case *events.Message:
		w.handleMessage(e)
	case *events.Receipt:
		w.handleReceipt(e)
	case *events.HistorySync:
		w.handleHistorySync(e)
	case *events.ChatPresence:
		// typing notification - ignore for now
	case *events.Presence:
		// presence update - ignore for now
	case *events.GroupInfo:
		// group info change - ignore for now
	}
}

func (w *WhatsmeowAdapter) handleQR(e *events.QR) {
	if len(e.Codes) == 0 {
		return
	}
	// Use the first QR code
	qrStr := e.Codes[0]

	// Generate QR code image as data URL
	qrImg, err := qrcode.Encode(qrStr, qrcode.Medium, 256)
	if err != nil {
		w.log.Error().Err(err).Msg("Failed to generate QR image")
		return
	}
	dataURL := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(qrImg))

	w.mu.Lock()
	w.qrCode = &dataURL
	w.setStatusUnsafe(StatusQRReady)
	cb := w.callbacks.OnQRCode
	w.mu.Unlock()

	if cb != nil {
		cb(dataURL)
	}
	if onState := w.getOnStateChanged(); onState != nil {
		onState(StatusQRReady)
	}
}

func (w *WhatsmeowAdapter) handleConnected() {
	w.mu.Lock()
	w.qrCode = nil
	phone := ""
	pushName := ""
	if w.client != nil && w.client.Store != nil {
		if w.client.Store.ID != nil {
			phone = w.client.Store.ID.User
		}
		pushName = w.client.Store.PushName
	}
	w.phoneNumber = &phone
	w.pushName = &pushName
	w.setStatusUnsafe(StatusReady)
	cb := w.callbacks.OnReady
	w.mu.Unlock()

	w.log.Info().Str("phone", phone).Msg("Connected to WhatsApp")
	if cb != nil {
		cb(phone, pushName)
	}
	if onState := w.getOnStateChanged(); onState != nil {
		onState(StatusReady)
	}
}

func (w *WhatsmeowAdapter) handlePairSuccess(e *events.PairSuccess) {
	w.log.Info().Str("jid", e.ID.String()).Msg("Pairing successful")
}

func (w *WhatsmeowAdapter) handleDisconnected() {
	w.mu.Lock()
	if w.intentionalClose {
		w.setStatusUnsafe(StatusDisconnected)
		cb := w.callbacks.OnDisconnected
		w.mu.Unlock()
		if cb != nil {
			cb("intentional disconnect")
		}
		return
	}
	w.setStatusUnsafe(StatusDisconnected)
	cb := w.callbacks.OnDisconnected
	w.mu.Unlock()

	w.log.Warn().Msg("Disconnected from WhatsApp")
	if cb != nil {
		cb("connection lost")
	}
}

func (w *WhatsmeowAdapter) handleMessage(e *events.Message) {
	msg := w.mapIncomingMessage(e)
	if msg == nil {
		return
	}

	if e.Info.IsFromMe {
		w.mu.RLock()
		cb := w.callbacks.OnMessageCreate
		w.mu.RUnlock()
		if cb != nil {
			cb(*msg)
		}
		return
	}

	w.mu.RLock()
	cb := w.callbacks.OnMessage
	w.mu.RUnlock()
	if cb != nil {
		cb(*msg)
	}
}

func (w *WhatsmeowAdapter) handleReceipt(e *events.Receipt) {
	var status DeliveryStatus
	switch e.Type {
	case types.ReceiptTypeDelivered:
		status = DeliveryDelivered
	case types.ReceiptTypeRead, types.ReceiptTypePlayed:
		status = DeliveryRead
	default:
		return
	}

	for _, msgID := range e.MessageIDs {
		w.mu.RLock()
		cb := w.callbacks.OnMessageAck
		w.mu.RUnlock()
		if cb != nil {
			cb(string(msgID), status)
		}
	}
}

func (w *WhatsmeowAdapter) handleHistorySync(e *events.HistorySync) {
	if e.Data == nil {
		return
	}
	messages := make([]IncomingMessage, 0)
	// History sync data processing
	if len(messages) > 0 {
		w.mu.RLock()
		cb := w.callbacks.OnHistoryMessages
		w.mu.RUnlock()
		if cb != nil {
			cb(messages)
		}
	}
}

// mapIncomingMessage converts a whatsmeow Message event to our IncomingMessage.
func (w *WhatsmeowAdapter) mapIncomingMessage(e *events.Message) *IncomingMessage {
	if e.Message == nil {
		return nil
	}

	msg := &IncomingMessage{
		ID:        e.Info.ID,
		Timestamp: e.Info.Timestamp.Unix(),
		FromMe:    e.Info.IsFromMe,
		IsGroup:   e.Info.IsGroup,
	}

	if !e.Info.IsFromMe {
		msg.From = e.Info.Sender.String()
		msg.ChatID = e.Info.Chat.String()
		msg.To = e.Info.Chat.String()
		if e.Info.IsGroup {
			msg.Author = e.Info.Sender.String()
			msg.From = e.Info.Chat.String()
		}
	} else {
		msg.From = e.Info.Sender.String()
		msg.To = e.Info.Chat.String()
		msg.ChatID = e.Info.Chat.String()
	}

	// Extract text content
	msg.Body = e.Message.GetConversation()
	if ext := e.Message.GetExtendedTextMessage(); ext != nil {
		msg.Body = ext.GetText()
		if ext.GetContextInfo() != nil {
			msg.MentionedIDs = ext.GetContextInfo().GetMentionedJID()
			if quoted := ext.GetContextInfo().GetQuotedMessage(); quoted != nil {
				msg.QuotedMessage = &QuotedMessage{
					ID:   ext.GetContextInfo().GetStanzaID(),
					Body: quoted.GetConversation(),
				}
			}
		}
	}

	// Determine message type
	switch {
	case e.Message.GetConversation() != "" || e.Message.GetExtendedTextMessage() != nil:
		msg.Type = MsgTypeText
	case e.Message.GetImageMessage() != nil:
		msg.Type = MsgTypeImage
		img := e.Message.GetImageMessage()
		msg.Body = img.GetCaption()
		msg.Media = &MediaInfo{
			Mimetype:  img.GetMimetype(),
			SizeBytes: int64(img.GetFileLength()),
		}
	case e.Message.GetVideoMessage() != nil:
		msg.Type = MsgTypeVideo
		vid := e.Message.GetVideoMessage()
		msg.Body = vid.GetCaption()
		msg.Media = &MediaInfo{
			Mimetype:  vid.GetMimetype(),
			SizeBytes: int64(vid.GetFileLength()),
		}
	case e.Message.GetAudioMessage() != nil:
		audio := e.Message.GetAudioMessage()
		if audio.GetPTT() {
			msg.Type = MsgTypeVoice
		} else {
			msg.Type = MsgTypeAudio
		}
		msg.Media = &MediaInfo{
			Mimetype:  audio.GetMimetype(),
			SizeBytes: int64(audio.GetFileLength()),
		}
	case e.Message.GetDocumentMessage() != nil:
		msg.Type = MsgTypeDocument
		doc := e.Message.GetDocumentMessage()
		msg.Body = doc.GetCaption()
		msg.Media = &MediaInfo{
			Mimetype: doc.GetMimetype(),
			Filename: doc.GetFileName(),
			SizeBytes: int64(doc.GetFileLength()),
		}
	case e.Message.GetStickerMessage() != nil:
		msg.Type = MsgTypeSticker
		st := e.Message.GetStickerMessage()
		msg.Media = &MediaInfo{
			Mimetype: st.GetMimetype(),
		}
	case e.Message.GetLocationMessage() != nil:
		msg.Type = MsgTypeLocation
		loc := e.Message.GetLocationMessage()
		msg.Location = &LocationInfo{
			Latitude:  loc.GetDegreesLatitude(),
			Longitude: loc.GetDegreesLongitude(),
		}
	case e.Message.GetContactMessage() != nil:
		msg.Type = MsgTypeContact
	// Call log messages don't have a direct getter
	default:
		msg.Type = MsgTypeUnknown
	}

	return msg
}

// --- IWhatsAppEngine Implementation ---

func (w *WhatsmeowAdapter) Disconnect() error {
	w.mu.Lock()
	w.intentionalClose = true
	w.mu.Unlock()

	if w.client != nil {
		w.client.Disconnect()
	}
	w.setStatus(StatusDisconnected)
	return nil
}

func (w *WhatsmeowAdapter) Logout() error {
	w.mu.Lock()
	w.intentionalClose = true
	w.mu.Unlock()

	if w.client != nil {
		if err := w.client.Logout(w.ctx); err != nil {
			w.log.Warn().Err(err).Msg("Logout failed")
		}
		w.client.Disconnect()
	}
	w.setStatus(StatusDisconnected)
	return nil
}

func (w *WhatsmeowAdapter) Destroy() error {
	w.mu.Lock()
	w.intentionalClose = true
	w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
	}
	if w.client != nil {
		w.client.Disconnect()
	}
	w.setStatus(StatusDisconnected)
	return nil
}

func (w *WhatsmeowAdapter) ForceDestroy() error {
	return w.Destroy()
}

func (w *WhatsmeowAdapter) GetStatus() EngineStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.status
}

func (w *WhatsmeowAdapter) GetQRCode() *string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.qrCode
}

func (w *WhatsmeowAdapter) RequestPairingCode(phoneNumber string) (string, error) {
	w.mu.RLock()
	client := w.client
	w.mu.RUnlock()

	if client == nil {
		return "", fmt.Errorf("engine not initialized")
	}

	code, err := client.PairPhone(w.ctx, phoneNumber, true, whatsmeow.PairClientChrome, "OpenWA")
	if err != nil {
		return "", err
	}
	return code, nil
}

func (w *WhatsmeowAdapter) GetPhoneNumber() *string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.phoneNumber
}

func (w *WhatsmeowAdapter) GetPushName() *string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.pushName
}

func (w *WhatsmeowAdapter) SendTextMessage(chatID, text string, mentions []string) (MessageResult, error) {
	client, jid, err := w.getClientAndJID(chatID)
	if err != nil {
		return MessageResult{}, err
	}

	var msg *waProto.Message
	if len(mentions) > 0 {
		msg = &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: proto.String(text),
				ContextInfo: &waProto.ContextInfo{
					MentionedJID: mentions,
				},
			},
		}
	} else {
		msg = &waProto.Message{
			Conversation: proto.String(text),
		}
	}

	resp, err := client.SendMessage(w.ctx, jid, msg)
	if err != nil {
		return MessageResult{}, err
	}

	return MessageResult{
		ID:        resp.ID,
		Timestamp: resp.Timestamp.Unix(),
	}, nil
}

func (w *WhatsmeowAdapter) sendMedia(chatID string, media MediaInput, msgType whatsmeow.MediaType, buildMsg func(*waProto.Message, *whatsmeow.UploadResponse)) (MessageResult, error) {
	client, jid, err := w.getClientAndJID(chatID)
	if err != nil {
		return MessageResult{}, err
	}

	var plaintext []byte
	if media.URL != "" {
		resp, err := http.Get(media.URL)
		if err != nil {
			return MessageResult{}, fmt.Errorf("failed to fetch media URL: %w", err)
		}
		defer resp.Body.Close()
		plaintext, err = io.ReadAll(resp.Body)
		if err != nil {
			return MessageResult{}, fmt.Errorf("failed to read media from URL: %w", err)
		}
	} else if media.Data != nil {
		plaintext = media.Data
	} else {
		return MessageResult{}, fmt.Errorf("no media data or URL provided")
	}

	// Upload media
	uploadResp, err := client.Upload(w.ctx, plaintext, msgType)
	if err != nil {
		return MessageResult{}, fmt.Errorf("failed to upload media: %w", err)
	}

	msg := &waProto.Message{}
	buildMsg(msg, &uploadResp)

	resp, err := client.SendMessage(w.ctx, jid, msg)
	if err != nil {
		return MessageResult{}, err
	}

	return MessageResult{
		ID:        resp.ID,
		Timestamp: resp.Timestamp.Unix(),
	}, nil
}

func (w *WhatsmeowAdapter) SendImageMessage(chatID string, media MediaInput) (MessageResult, error) {
	return w.sendMedia(chatID, media, whatsmeow.MediaImage, func(msg *waProto.Message, upload *whatsmeow.UploadResponse) {
		msg.ImageMessage = &waProto.ImageMessage{
			URL:           &upload.URL,
			DirectPath:    &upload.DirectPath,
			MediaKey:      upload.MediaKey,
			Mimetype:      &media.Mimetype,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    &upload.FileLength,
			Caption:       &media.Caption,
		}
	})
}

func (w *WhatsmeowAdapter) SendVideoMessage(chatID string, media MediaInput) (MessageResult, error) {
	return w.sendMedia(chatID, media, whatsmeow.MediaVideo, func(msg *waProto.Message, upload *whatsmeow.UploadResponse) {
		msg.VideoMessage = &waProto.VideoMessage{
			URL:           &upload.URL,
			DirectPath:    &upload.DirectPath,
			MediaKey:      upload.MediaKey,
			Mimetype:      &media.Mimetype,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    &upload.FileLength,
			Caption:       &media.Caption,
		}
	})
}

func (w *WhatsmeowAdapter) SendAudioMessage(chatID string, media MediaInput) (MessageResult, error) {
	return w.sendMedia(chatID, media, whatsmeow.MediaAudio, func(msg *waProto.Message, upload *whatsmeow.UploadResponse) {
		msg.AudioMessage = &waProto.AudioMessage{
			URL:           &upload.URL,
			DirectPath:    &upload.DirectPath,
			MediaKey:      upload.MediaKey,
			Mimetype:      &media.Mimetype,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    &upload.FileLength,
			PTT:           proto.Bool(false),
		}
	})
}

func (w *WhatsmeowAdapter) SendDocumentMessage(chatID string, media MediaInput) (MessageResult, error) {
	return w.sendMedia(chatID, media, whatsmeow.MediaDocument, func(msg *waProto.Message, upload *whatsmeow.UploadResponse) {
		msg.DocumentMessage = &waProto.DocumentMessage{
			URL:           &upload.URL,
			DirectPath:    &upload.DirectPath,
			MediaKey:      upload.MediaKey,
			Mimetype:      &media.Mimetype,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    &upload.FileLength,
			FileName:      &media.Filename,
			Caption:       &media.Caption,
		}
	})
}

func (w *WhatsmeowAdapter) SendLocationMessage(chatID string, location LocationInput) (MessageResult, error) {
	client, jid, err := w.getClientAndJID(chatID)
	if err != nil {
		return MessageResult{}, err
	}

	msg := &waProto.Message{
		LocationMessage: &waProto.LocationMessage{
			DegreesLatitude:  proto.Float64(location.Latitude),
			DegreesLongitude: proto.Float64(location.Longitude),
			Name:             proto.String(location.Description),
			Address:          proto.String(location.Address),
		},
	}

	resp, err := client.SendMessage(w.ctx, jid, msg)
	if err != nil {
		return MessageResult{}, err
	}

	return MessageResult{
		ID:        resp.ID,
		Timestamp: resp.Timestamp.Unix(),
	}, nil
}

func (w *WhatsmeowAdapter) SendContactMessage(chatID string, contact ContactCard) (MessageResult, error) {
	client, jid, err := w.getClientAndJID(chatID)
	if err != nil {
		return MessageResult{}, err
	}

	vcard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nFN:%s\nTEL;TYPE=CELL:%s\nEND:VCARD", contact.Name, contact.Number)

	msg := &waProto.Message{
		ContactMessage: &waProto.ContactMessage{
			DisplayName: proto.String(contact.Name),
			Vcard:       proto.String(vcard),
		},
	}

	resp, err := client.SendMessage(w.ctx, jid, msg)
	if err != nil {
		return MessageResult{}, err
	}

	return MessageResult{
		ID:        resp.ID,
		Timestamp: resp.Timestamp.Unix(),
	}, nil
}

func (w *WhatsmeowAdapter) SendStickerMessage(chatID string, media MediaInput) (MessageResult, error) {
	return w.sendMedia(chatID, media, whatsmeow.MediaImage, func(msg *waProto.Message, upload *whatsmeow.UploadResponse) {
		msg.StickerMessage = &waProto.StickerMessage{
			URL:           &upload.URL,
			DirectPath:    &upload.DirectPath,
			MediaKey:      upload.MediaKey,
			Mimetype:      &media.Mimetype,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    &upload.FileLength,
		}
	})
}

func (w *WhatsmeowAdapter) ReplyToMessage(chatID, quotedMsgID, text string) (MessageResult, error) {
	client, jid, err := w.getClientAndJID(chatID)
	if err != nil {
		return MessageResult{}, err
	}

	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      &quotedMsgID,
				Participant:   proto.String(jid.String()),
				QuotedMessage: &waProto.Message{},
			},
		},
	}

	resp, err := client.SendMessage(w.ctx, jid, msg)
	if err != nil {
		return MessageResult{}, err
	}

	return MessageResult{
		ID:        resp.ID,
		Timestamp: resp.Timestamp.Unix(),
	}, nil
}

func (w *WhatsmeowAdapter) ForwardMessage(fromChatID, toChatID, messageID string) (MessageResult, error) {
	return MessageResult{}, fmt.Errorf("forward not yet implemented")
}

func (w *WhatsmeowAdapter) ReactToMessage(chatID, messageID, emoji string) error {
	client, jid, err := w.getClientAndJID(chatID)
	if err != nil {
		return err
	}

	msg := &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{
			Key: &waCommon.MessageKey{
				RemoteJID: &chatID,
				FromMe:    proto.Bool(true),
				ID:        &messageID,
			},
			Text:              proto.String(emoji),
			GroupingKey:       proto.String(""),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	}

	_, err = client.SendMessage(w.ctx, jid, msg)
	return err
}

func (w *WhatsmeowAdapter) GetMessageReactions(chatID, messageID string) ([]MessageReaction, error) {
	return nil, fmt.Errorf("GetMessageReactions not yet implemented")
}

func (w *WhatsmeowAdapter) GetContacts() ([]Contact, error) {
	client := w.getClient()
	if client == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	allContacts, err := client.Store.Contacts.GetAllContacts(w.ctx)
	if err != nil {
		return nil, err
	}

	contacts := make([]Contact, 0, len(allContacts))
	for jid, info := range allContacts {
		contacts = append(contacts, Contact{
			ID:       jid.String(),
			Number:   jid.User,
			PushName: info.PushName,
		})
	}

	return contacts, nil
}

func (w *WhatsmeowAdapter) GetContactByID(contactID string) (*Contact, error) {
	client := w.getClient()
	if client == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(contactID)
	if err != nil {
		return nil, err
	}

	info, err := client.Store.Contacts.GetContact(w.ctx, jid)
	if err != nil {
		return nil, err
	}

	return &Contact{
		ID:       jid.String(),
		Number:   jid.User,
		PushName: info.PushName,
	}, nil
}

func (w *WhatsmeowAdapter) CheckNumberExists(number string) (bool, error) {
	client := w.getClient()
	if client == nil {
		return false, fmt.Errorf("engine not initialized")
	}

	results, err := client.IsOnWhatsApp(w.ctx, []string{number})
	if err != nil {
		return false, err
	}
	return len(results) > 0 && results[0].IsIn, nil
}

func (w *WhatsmeowAdapter) GetNumberID(number string) (*string, error) {
	client := w.getClient()
	if client == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	results, err := client.IsOnWhatsApp(w.ctx, []string{number})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 || !results[0].IsIn {
		return nil, nil
	}
	result := results[0].JID.String()
	return &result, nil
}

func (w *WhatsmeowAdapter) ResolveContactPhone(contactID string) (*string, error) {
	client := w.getClient()
	if client == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(contactID)
	if err != nil {
		return nil, err
	}

	info, err := client.Store.Contacts.GetContact(w.ctx, jid)
	if err == nil && info.Found {
		return &jid.User, nil
	}
	return nil, nil
}

func (w *WhatsmeowAdapter) GetGroups() ([]Group, error) {
	client := w.getClient()
	if client == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	groups, err := client.GetJoinedGroups(w.ctx)
	if err != nil {
		return nil, err
	}

	result := make([]Group, len(groups))
	for i, g := range groups {
		result[i] = Group{
			ID:                g.JID.String(),
			Name:              g.Name,
			ParticipantsCount: len(g.Participants),
		}
	}

	return result, nil
}

func (w *WhatsmeowAdapter) GetGroupInfo(groupID string) (*GroupInfo, error) {
	client := w.getClient()
	if client == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(groupID)
	if err != nil {
		return nil, err
	}

	info, err := client.GetGroupInfo(w.ctx, jid)
	if err != nil {
		return nil, err
	}

	participants := make([]GroupParticipant, len(info.Participants))
	for i, p := range info.Participants {
		participants[i] = GroupParticipant{
			ID:      p.JID.String(),
			Number:  p.JID.User,
			IsAdmin: p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
		}
	}

	return &GroupInfo{
		ID:           info.JID.String(),
		Name:         info.Name,
		Description:  info.Topic,
		Owner:        info.OwnerJID.String(),
		Participants: participants,
		IsReadOnly:   info.IsAnnounce,
		IsAnnounce:   info.IsAnnounce,
	}, nil
}

func (w *WhatsmeowAdapter) CreateGroup(name string, participants []string) (Group, error) {
	client := w.getClient()
	if client == nil {
		return Group{}, fmt.Errorf("engine not initialized")
	}

	jids := make([]types.JID, len(participants))
	for i, p := range participants {
		jid, err := types.ParseJID(p)
		if err != nil {
			return Group{}, err
		}
		jids[i] = jid
	}

	info, err := client.CreateGroup(w.ctx, whatsmeow.ReqCreateGroup{Name: name, Participants: jids})
	if err != nil {
		return Group{}, err
	}

	return Group{
		ID:   info.JID.String(),
		Name: info.Name,
	}, nil
}

func (w *WhatsmeowAdapter) AddParticipants(groupID string, participants []string) error {
	_, err := w.updateParticipants(groupID, participants, whatsmeow.ParticipantChangeAdd)
	return err
}

func (w *WhatsmeowAdapter) RemoveParticipants(groupID string, participants []string) error {
	_, err := w.updateParticipants(groupID, participants, whatsmeow.ParticipantChangeRemove)
	return err
}

func (w *WhatsmeowAdapter) PromoteParticipants(groupID string, participants []string) error {
	_, err := w.updateParticipants(groupID, participants, whatsmeow.ParticipantChangePromote)
	return err
}

func (w *WhatsmeowAdapter) DemoteParticipants(groupID string, participants []string) error {
	_, err := w.updateParticipants(groupID, participants, whatsmeow.ParticipantChangeDemote)
	return err
}

func (w *WhatsmeowAdapter) updateParticipants(groupID string, participants []string, action whatsmeow.ParticipantChange) ([]types.GroupParticipant, error) {
	client := w.getClient()
	if client == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(groupID)
	if err != nil {
		return nil, err
	}

	jids := make([]types.JID, len(participants))
	for i, p := range participants {
		pjid, err := types.ParseJID(p)
		if err != nil {
			return nil, err
		}
		jids[i] = pjid
	}

	return client.UpdateGroupParticipants(w.ctx, jid, jids, action)
}

func (w *WhatsmeowAdapter) LeaveGroup(groupID string) error {
	client := w.getClient()
	if client == nil {
		return fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(groupID)
	if err != nil {
		return err
	}

	return client.LeaveGroup(w.ctx, jid)
}

func (w *WhatsmeowAdapter) SetGroupSubject(groupID, subject string) error {
	client := w.getClient()
	if client == nil {
		return fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(groupID)
	if err != nil {
		return err
	}

	return client.SetGroupName(w.ctx, jid, subject)
}

func (w *WhatsmeowAdapter) SetGroupDescription(groupID, description string) error {
	client := w.getClient()
	if client == nil {
		return fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(groupID)
	if err != nil {
		return err
	}

	return client.SetGroupTopic(w.ctx, jid, "", "", description)
}

func (w *WhatsmeowAdapter) GetGroupInviteCode(groupID string) (string, error) {
	client := w.getClient()
	if client == nil {
		return "", fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(groupID)
	if err != nil {
		return "", err
	}

	code, err := client.GetGroupInviteLink(w.ctx, jid, false)
	return code, err
}

func (w *WhatsmeowAdapter) RevokeGroupInviteCode(groupID string) (string, error) {
	client := w.getClient()
	if client == nil {
		return "", fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(groupID)
	if err != nil {
		return "", err
	}

	code, err := client.GetGroupInviteLink(w.ctx, jid, true)
	return code, err
}

func (w *WhatsmeowAdapter) DeleteMessage(chatID, messageID string, forEveryone bool) error {
	client, jid, err := w.getClientAndJID(chatID)
	if err != nil {
		return err
	}

	if forEveryone {
		_, err = client.SendMessage(w.ctx, jid, &waProto.Message{
			ProtocolMessage: &waProto.ProtocolMessage{
				Key: &waCommon.MessageKey{
					RemoteJID: &chatID,
					FromMe:    proto.Bool(true),
					ID:        &messageID,
				},
				Type: waProto.ProtocolMessage_REVOKE.Enum(),
			},
		})
	} else {
		return fmt.Errorf("delete for me not yet implemented")
	}

	return err
}

func (w *WhatsmeowAdapter) GetChatHistory(chatID string, limit int, includeMedia bool) ([]IncomingMessage, error) {
	return nil, fmt.Errorf("GetChatHistory not yet implemented")
}

func (w *WhatsmeowAdapter) GetProfilePicture(contactID string) (*string, error) {
	client := w.getClient()
	if client == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(contactID)
	if err != nil {
		return nil, err
	}

	pic, err := client.GetProfilePictureInfo(w.ctx, jid, nil)
	if err != nil {
		return nil, err
	}

	if pic == nil {
		return nil, nil
	}
	return &pic.URL, nil
}

func (w *WhatsmeowAdapter) BlockContact(contactID string) error {
	client := w.getClient()
	if client == nil {
		return fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(contactID)
	if err != nil {
		return err
	}

	_, err = client.UpdateBlocklist(w.ctx, jid, events.BlocklistChangeActionBlock)
	return err
}

func (w *WhatsmeowAdapter) UnblockContact(contactID string) error {
	client := w.getClient()
	if client == nil {
		return fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(contactID)
	if err != nil {
		return err
	}

	_, err = client.UpdateBlocklist(w.ctx, jid, events.BlocklistChangeActionUnblock)
	return err
}

func (w *WhatsmeowAdapter) GetLabels() ([]Label, error) {
	return nil, fmt.Errorf("labels not yet implemented")
}

func (w *WhatsmeowAdapter) GetLabelByID(labelID string) (*Label, error) {
	return nil, fmt.Errorf("labels not yet implemented")
}

func (w *WhatsmeowAdapter) GetChatLabels(chatID string) ([]Label, error) {
	return nil, fmt.Errorf("labels not yet implemented")
}

func (w *WhatsmeowAdapter) AddLabelToChat(chatID, labelID string) error {
	return fmt.Errorf("labels not yet implemented")
}

func (w *WhatsmeowAdapter) RemoveLabelFromChat(chatID, labelID string) error {
	return fmt.Errorf("labels not yet implemented")
}

func (w *WhatsmeowAdapter) GetSubscribedChannels() ([]Channel, error) {
	return nil, fmt.Errorf("channels not yet implemented")
}

func (w *WhatsmeowAdapter) GetChannelByID(channelID string) (*Channel, error) {
	return nil, fmt.Errorf("channels not yet implemented")
}

func (w *WhatsmeowAdapter) SubscribeToChannel(inviteCode string) (Channel, error) {
	return Channel{}, fmt.Errorf("channels not yet implemented")
}

func (w *WhatsmeowAdapter) UnsubscribeFromChannel(channelID string) error {
	return fmt.Errorf("channels not yet implemented")
}

func (w *WhatsmeowAdapter) GetChannelMessages(channelID string, limit int) ([]ChannelMessage, error) {
	return nil, fmt.Errorf("channels not yet implemented")
}

func (w *WhatsmeowAdapter) GetContactStatuses() ([]Status, error) {
	return nil, fmt.Errorf("status not yet implemented")
}

func (w *WhatsmeowAdapter) GetContactStatus(contactID string) ([]Status, error) {
	return nil, fmt.Errorf("status not yet implemented")
}

func (w *WhatsmeowAdapter) PostTextStatus(text string, opts StatusPostOptions) (StatusResult, error) {
	client := w.getClient()
	if client == nil {
		return StatusResult{}, fmt.Errorf("engine not initialized")
	}

	msg := &waProto.Message{
		Conversation: proto.String(text),
	}

	resp, err := client.SendMessage(w.ctx, types.StatusBroadcastJID, msg)
	if err != nil {
		return StatusResult{}, err
	}

	return StatusResult{
		StatusID:  resp.ID,
		Timestamp: resp.Timestamp.Unix(),
		ExpiresAt: resp.Timestamp.Add(24 * time.Hour).Unix(),
	}, nil
}

func (w *WhatsmeowAdapter) PostImageStatus(media MediaInput, opts StatusPostOptions) (StatusResult, error) {
	return StatusResult{}, fmt.Errorf("media status not yet implemented")
}

func (w *WhatsmeowAdapter) PostVideoStatus(media MediaInput, opts StatusPostOptions) (StatusResult, error) {
	return StatusResult{}, fmt.Errorf("media status not yet implemented")
}

func (w *WhatsmeowAdapter) DeleteStatus(statusID string) error {
	client := w.getClient()
	if client == nil {
		return fmt.Errorf("engine not initialized")
	}

	_, err := client.SendMessage(w.ctx, types.StatusBroadcastJID, &waProto.Message{
		ProtocolMessage: &waProto.ProtocolMessage{
			Key: &waCommon.MessageKey{
				RemoteJID: proto.String("status@broadcast"),
				FromMe:    proto.Bool(true),
				ID:        &statusID,
			},
			Type: waProto.ProtocolMessage_REVOKE.Enum(),
		},
	})
	return err
}

func (w *WhatsmeowAdapter) GetCatalog() (*Catalog, error) {
	return nil, fmt.Errorf("catalog not yet implemented")
}

func (w *WhatsmeowAdapter) GetProducts(opts ProductQueryOptions) (PaginatedProducts, error) {
	return PaginatedProducts{}, fmt.Errorf("products not yet implemented")
}

func (w *WhatsmeowAdapter) GetProduct(productID string) (*Product, error) {
	return nil, fmt.Errorf("products not yet implemented")
}

func (w *WhatsmeowAdapter) SendProduct(chatID, productID, body string) (MessageResult, error) {
	return MessageResult{}, fmt.Errorf("products not yet implemented")
}

func (w *WhatsmeowAdapter) SendCatalog(chatID, body string) (MessageResult, error) {
	return MessageResult{}, fmt.Errorf("catalog not yet implemented")
}

func (w *WhatsmeowAdapter) GetChats() ([]ChatSummary, error) {
	client := w.getClient()
	if client == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	allContacts, err := client.Store.Contacts.GetAllContacts(w.ctx)
	if err != nil {
		return nil, err
	}

	chats := make([]ChatSummary, 0, len(allContacts))
	for jid, info := range allContacts {
		chats = append(chats, ChatSummary{
			ID:      jid.String(),
			Name:    info.PushName,
			IsGroup: jid.Server == types.GroupServer,
		})
	}

	return chats, nil
}

func (w *WhatsmeowAdapter) SendSeen(chatID string) (bool, error) {
	client, jid, err := w.getClientAndJID(chatID)
	if err != nil {
		return false, err
	}

	err = client.MarkRead(w.ctx, []types.MessageID{types.MessageID(chatID)}, time.Now(), jid, jid)
	return err == nil, err
}

func (w *WhatsmeowAdapter) MarkUnread(chatID string) (bool, error) {
	return false, fmt.Errorf("mark unread not yet implemented")
}

func (w *WhatsmeowAdapter) DeleteChat(chatID string) (bool, error) {
	_, _, err := w.getClientAndJID(chatID)
	if err != nil {
		return false, err
	}

	// whatsmeow doesn't have a direct delete chat method
	return false, fmt.Errorf("delete chat not yet implemented")
}

func (w *WhatsmeowAdapter) SendChatState(chatID string, state ChatState) error {
	client, jid, err := w.getClientAndJID(chatID)
	if err != nil {
		return err
	}

	switch state {
	case ChatStateTyping:
		return client.SendChatPresence(w.ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	case ChatStateRecording:
		return client.SendChatPresence(w.ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaAudio)
	case ChatStatePaused:
		return client.SendChatPresence(w.ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)
	}
	return nil
}

// --- Helper methods ---

func (w *WhatsmeowAdapter) getClient() *whatsmeow.Client {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.client
}

func (w *WhatsmeowAdapter) getClientAndJID(chatID string) (*whatsmeow.Client, types.JID, error) {
	w.mu.RLock()
	client := w.client
	w.mu.RUnlock()

	if client == nil {
		return nil, types.JID{}, fmt.Errorf("engine not initialized")
	}

	jid, err := types.ParseJID(chatID)
	if err != nil {
		return nil, types.JID{}, fmt.Errorf("invalid JID: %w", err)
	}

	return client, jid, nil
}

func (w *WhatsmeowAdapter) setStatus(status EngineStatus) {
	w.mu.Lock()
	w.setStatusUnsafe(status)
	w.mu.Unlock()
}

func (w *WhatsmeowAdapter) setStatusUnsafe(status EngineStatus) {
	w.status = status
}

func (w *WhatsmeowAdapter) getOnStateChanged() func(EngineStatus) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.callbacks.OnStateChanged
}

func (w *WhatsmeowAdapter) fireError(reason string) {
	w.mu.RLock()
	cb := w.callbacks.OnError
	w.mu.RUnlock()
	if cb != nil {
		cb(reason)
	}
}
