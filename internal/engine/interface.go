// Package engine defines the engine-neutral interface for WhatsApp communication.
// This is the Go port of the TypeScript IWhatsAppEngine interface from OpenWA.
package engine

// EngineStatus represents the current state of a WhatsApp engine connection.
type EngineStatus string

const (
	StatusDisconnected   EngineStatus = "disconnected"
	StatusInitializing   EngineStatus = "initializing"
	StatusQRReady        EngineStatus = "qr_ready"
	StatusAuthenticating EngineStatus = "authenticating"
	StatusReady          EngineStatus = "ready"
	StatusFailed         EngineStatus = "failed"
)

// MessageType is an engine-neutral message type vocabulary.
type MessageType string

const (
	MsgTypeText     MessageType = "text"
	MsgTypeImage    MessageType = "image"
	MsgTypeVideo    MessageType = "video"
	MsgTypeAudio    MessageType = "audio"
	MsgTypeVoice    MessageType = "voice"
	MsgTypeDocument MessageType = "document"
	MsgTypeSticker  MessageType = "sticker"
	MsgTypeLocation MessageType = "location"
	MsgTypeContact  MessageType = "contact"
	MsgTypeCall     MessageType = "call"
	MsgTypeRevoked  MessageType = "revoked"
	MsgTypeUnknown  MessageType = "unknown"
)

// DeliveryStatus is an engine-neutral message delivery state.
type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliverySent      DeliveryStatus = "sent"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryRead      DeliveryStatus = "read"
	DeliveryFailed    DeliveryStatus = "failed"
)

// ChatState represents typing/recording presence indicator state.
type ChatState string

const (
	ChatStateTyping    ChatState = "typing"
	ChatStateRecording ChatState = "recording"
	ChatStatePaused    ChatState = "paused"
)

// MessageResult contains the result of a sent message.
type MessageResult struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
}

// MediaInput represents media to be sent.
type MediaInput struct {
	Mimetype string   `json:"mimetype"`
	Data     []byte   `json:"data,omitempty"`
	Filename string   `json:"filename,omitempty"`
	Caption  string   `json:"caption,omitempty"`
	Mentions []string `json:"mentions,omitempty"`
	URL      string   `json:"url,omitempty"`
}

// IncomingMessage represents a received WhatsApp message (engine-neutral).
type IncomingMessage struct {
	ID                string         `json:"id"`
	From              string         `json:"from"`
	To                string         `json:"to"`
	ChatID            string         `json:"chatId"`
	Body              string         `json:"body"`
	Type              MessageType    `json:"type"`
	Timestamp         int64          `json:"timestamp"`
	FromMe            bool           `json:"fromMe"`
	IsGroup           bool           `json:"isGroup"`
	IsStatusBroadcast bool           `json:"isStatusBroadcast,omitempty"`
	Author            string         `json:"author,omitempty"`
	MentionedIDs      []string       `json:"mentionedIds,omitempty"`
	EphemeralDuration int            `json:"ephemeralDuration,omitempty"`
	Call              *CallInfo      `json:"call,omitempty"`
	Media             *MediaInfo     `json:"media,omitempty"`
	QuotedMessage     *QuotedMessage `json:"quotedMessage,omitempty"`
	Location          *LocationInfo  `json:"location,omitempty"`
	Contact           *MessageContact `json:"contact,omitempty"`
}

// CallInfo details a call_log message.
type CallInfo struct {
	Video  bool `json:"video"`
	Missed bool `json:"missed"`
}

// MediaInfo describes attached media in an incoming message.
type MediaInfo struct {
	Mimetype  string `json:"mimetype"`
	Filename  string `json:"filename,omitempty"`
	Data      string `json:"data,omitempty"`
	Omitted   bool   `json:"omitted,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
}

// QuotedMessage references a replied-to message.
type QuotedMessage struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

// LocationInfo describes a location message.
type LocationInfo struct {
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Description string  `json:"description,omitempty"`
	Address     string  `json:"address,omitempty"`
	URL         string  `json:"url,omitempty"`
}

// MessageContact contains sender contact info.
type MessageContact struct {
	ID           string `json:"id,omitempty"`
	Number       string `json:"number,omitempty"`
	Name         string `json:"name,omitempty"`
	PushName     string `json:"pushName,omitempty"`
	ShortName    string `json:"shortName,omitempty"`
	IsMyContact  bool   `json:"isMyContact,omitempty"`
	IsWAContact  bool   `json:"isWAContact,omitempty"`
	IsBusiness   bool   `json:"isBusiness,omitempty"`
	VerifiedName string `json:"verifiedName,omitempty"`
	IsBlocked    bool   `json:"isBlocked,omitempty"`
}

// Contact is a WhatsApp contact.
type Contact struct {
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	PushName     string `json:"pushName,omitempty"`
	Number       string `json:"number"`
	IsMyContact  bool   `json:"isMyContact"`
	IsBlocked    bool   `json:"isBlocked"`
	ProfilePicURL string `json:"profilePicUrl,omitempty"`
}

// Group is a WhatsApp group summary.
type Group struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	ParticipantsCount int    `json:"participantsCount,omitempty"`
	IsAdmin           bool   `json:"isAdmin,omitempty"`
	LinkedParentJID   string `json:"linkedParentJID,omitempty"`
}

// GroupParticipant is a member of a group.
type GroupParticipant struct {
	ID          string `json:"id"`
	Number      string `json:"number"`
	Name        string `json:"name,omitempty"`
	IsAdmin     bool   `json:"isAdmin"`
	IsSuperAdmin bool  `json:"isSuperAdmin"`
}

// GroupInfo is detailed group information.
type GroupInfo struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Description     string             `json:"description,omitempty"`
	Owner           string             `json:"owner,omitempty"`
	CreatedAt       int64              `json:"createdAt,omitempty"`
	Participants    []GroupParticipant `json:"participants"`
	IsReadOnly      bool               `json:"isReadOnly"`
	IsAnnounce      bool               `json:"isAnnounce"`
	LinkedParentJID string             `json:"linkedParentJID,omitempty"`
}

// LocationInput is used for sending a location message.
type LocationInput struct {
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Description string  `json:"description,omitempty"`
	Address     string  `json:"address,omitempty"`
}

// ContactCard is used for sending a contact message.
type ContactCard struct {
	Name   string `json:"name"`
	Number string `json:"number"`
}

// ReactionEvent represents a message reaction event.
type ReactionEvent struct {
	MessageID string `json:"messageId"`
	ChatID    string `json:"chatId"`
	Reaction  string `json:"reaction"`
	SenderID  string `json:"senderId"`
}

// RevokedMessage represents a remotely revoked message.
type RevokedMessage struct {
	ID        string      `json:"id"`
	ChatID    string      `json:"chatId"`
	From      string      `json:"from"`
	To        string      `json:"to"`
	Type      MessageType `json:"type"`
	Body      string      `json:"body"`
	Timestamp int64       `json:"timestamp"`
}

// ChatSummary is a lightweight chat representation.
type ChatSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IsGroup     bool   `json:"isGroup"`
	UnreadCount int    `json:"unreadCount"`
	Timestamp   int64  `json:"timestamp"`
	LastMessage string `json:"lastMessage,omitempty"`
}

// Label is a WhatsApp Business label.
type Label struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	HexColor string `json:"hexColor"`
}

// Channel is a WhatsApp channel/newsletter.
type Channel struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	InviteCode      string `json:"inviteCode,omitempty"`
	SubscriberCount int    `json:"subscriberCount,omitempty"`
	Picture         string `json:"picture,omitempty"`
	Verified        bool   `json:"verified,omitempty"`
}

// ChannelMessage is a message from a channel.
type ChannelMessage struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	Timestamp int64  `json:"timestamp"`
	HasMedia  bool   `json:"hasMedia"`
	MediaURL  string `json:"mediaUrl,omitempty"`
}

// StatusPostOptions configures a status/story post.
type StatusPostOptions struct {
	Recipients      []string `json:"recipients"`
	BackgroundColor string   `json:"backgroundColor,omitempty"`
	Font            int      `json:"font,omitempty"`
	Caption         string   `json:"caption,omitempty"`
}

// StatusResult is the result of posting a status.
type StatusResult struct {
	StatusID  string `json:"statusId"`
	Timestamp int64  `json:"timestamp"`
	ExpiresAt int64  `json:"expiresAt"`
}

// Status is a contact's status/story.
type Status struct {
	ID              string      `json:"id"`
	Contact         StatusContact `json:"contact"`
	Type            MessageType `json:"type"`
	Caption         string      `json:"caption,omitempty"`
	MediaURL        string      `json:"mediaUrl,omitempty"`
	BackgroundColor string      `json:"backgroundColor,omitempty"`
	Font            int         `json:"font,omitempty"`
	Timestamp       int64       `json:"timestamp"`
	ExpiresAt       int64       `json:"expiresAt"`
}

// StatusContact is the contact info in a status.
type StatusContact struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	PushName string `json:"pushName,omitempty"`
}

// Catalog is a WhatsApp Business catalog.
type Catalog struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ProductCount int    `json:"productCount"`
	URL          string `json:"url"`
}

// Product is a WhatsApp Business product.
type Product struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	Price          float64 `json:"price"`
	Currency       string `json:"currency"`
	PriceFormatted string `json:"priceFormatted"`
	ImageURL       string `json:"imageUrl,omitempty"`
	URL            string `json:"url"`
	IsAvailable    bool   `json:"isAvailable"`
	RetailerID     string `json:"retailerId,omitempty"`
}

// ProductQueryOptions for paginated product queries.
type ProductQueryOptions struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// PaginatedProducts contains a page of products.
type PaginatedProducts struct {
	Products   []Product `json:"products"`
	Pagination struct {
		Page       int `json:"page"`
		Limit      int `json:"limit"`
		Total      int `json:"total"`
		TotalPages int `json:"totalPages"`
	} `json:"pagination"`
}

// MessageReaction shows who reacted to a message with which emoji.
type MessageReaction struct {
	Emoji   string           `json:"emoji"`
	Senders []ReactionSender `json:"senders"`
}

// ReactionSender is a single user's reaction.
type ReactionSender struct {
	SenderID  string `json:"senderId"`
	Emoji     string `json:"emoji"`
	Timestamp int64  `json:"timestamp"`
}

// EngineEventCallbacks groups all event callbacks the engine can fire.
type EngineEventCallbacks struct {
	OnQRCode          func(qr string)
	OnReady           func(phone, pushName string)
	OnMessage         func(msg IncomingMessage)
	OnMessageCreate   func(msg IncomingMessage)
	OnMessageAck      func(messageID string, status DeliveryStatus)
	OnMessageRevoked  func(msg RevokedMessage)
	OnMessageReaction func(event ReactionEvent)
	OnHistoryMessages func(messages []IncomingMessage)
	OnDisconnected    func(reason string)
	OnStateChanged    func(state EngineStatus)
	OnError           func(reason string)
}

// IWhatsAppEngine is the engine-neutral interface for WhatsApp operations.
type IWhatsAppEngine interface {
	// Lifecycle
	Initialize(callbacks EngineEventCallbacks) error
	Disconnect() error
	Logout() error
	Destroy() error
	ForceDestroy() error

	// Status
	GetStatus() EngineStatus
	GetQRCode() *string
	RequestPairingCode(phoneNumber string) (string, error)
	GetPhoneNumber() *string
	GetPushName() *string

	// Messaging - Basic
	SendTextMessage(chatID, text string, mentions []string) (MessageResult, error)
	SendImageMessage(chatID string, media MediaInput) (MessageResult, error)
	SendVideoMessage(chatID string, media MediaInput) (MessageResult, error)
	SendAudioMessage(chatID string, media MediaInput) (MessageResult, error)
	SendDocumentMessage(chatID string, media MediaInput) (MessageResult, error)

	// Messaging - Extended
	SendLocationMessage(chatID string, location LocationInput) (MessageResult, error)
	SendContactMessage(chatID string, contact ContactCard) (MessageResult, error)
	SendStickerMessage(chatID string, media MediaInput) (MessageResult, error)

	// Reply & Forward
	ReplyToMessage(chatID, quotedMsgID, text string) (MessageResult, error)
	ForwardMessage(fromChatID, toChatID, messageID string) (MessageResult, error)

	// Reactions
	ReactToMessage(chatID, messageID, emoji string) error
	GetMessageReactions(chatID, messageID string) ([]MessageReaction, error)

	// Contacts
	GetContacts() ([]Contact, error)
	GetContactByID(contactID string) (*Contact, error)
	CheckNumberExists(number string) (bool, error)
	GetNumberID(number string) (*string, error)
	ResolveContactPhone(contactID string) (*string, error)

	// Groups
	GetGroups() ([]Group, error)
	GetGroupInfo(groupID string) (*GroupInfo, error)
	CreateGroup(name string, participants []string) (Group, error)
	AddParticipants(groupID string, participants []string) error
	RemoveParticipants(groupID string, participants []string) error
	PromoteParticipants(groupID string, participants []string) error
	DemoteParticipants(groupID string, participants []string) error
	LeaveGroup(groupID string) error
	SetGroupSubject(groupID, subject string) error
	SetGroupDescription(groupID, description string) error
	GetGroupInviteCode(groupID string) (string, error)
	RevokeGroupInviteCode(groupID string) (string, error)

	// Message Operations
	DeleteMessage(chatID, messageID string, forEveryone bool) error
	GetChatHistory(chatID string, limit int, includeMedia bool) ([]IncomingMessage, error)

	// Contact Extended
	GetProfilePicture(contactID string) (*string, error)
	BlockContact(contactID string) error
	UnblockContact(contactID string) error

	// Labels
	GetLabels() ([]Label, error)
	GetLabelByID(labelID string) (*Label, error)
	GetChatLabels(chatID string) ([]Label, error)
	AddLabelToChat(chatID, labelID string) error
	RemoveLabelFromChat(chatID, labelID string) error

	// Channels
	GetSubscribedChannels() ([]Channel, error)
	GetChannelByID(channelID string) (*Channel, error)
	SubscribeToChannel(inviteCode string) (Channel, error)
	UnsubscribeFromChannel(channelID string) error
	GetChannelMessages(channelID string, limit int) ([]ChannelMessage, error)

	// Status/Stories
	GetContactStatuses() ([]Status, error)
	GetContactStatus(contactID string) ([]Status, error)
	PostTextStatus(text string, opts StatusPostOptions) (StatusResult, error)
	PostImageStatus(media MediaInput, opts StatusPostOptions) (StatusResult, error)
	PostVideoStatus(media MediaInput, opts StatusPostOptions) (StatusResult, error)
	DeleteStatus(statusID string) error

	// Catalog
	GetCatalog() (*Catalog, error)
	GetProducts(opts ProductQueryOptions) (PaginatedProducts, error)
	GetProduct(productID string) (*Product, error)
	SendProduct(chatID, productID, body string) (MessageResult, error)
	SendCatalog(chatID, body string) (MessageResult, error)

	// Chats
	GetChats() ([]ChatSummary, error)
	SendSeen(chatID string) (bool, error)
	MarkUnread(chatID string) (bool, error)
	DeleteChat(chatID string) (bool, error)
	SendChatState(chatID string, state ChatState) error
}
