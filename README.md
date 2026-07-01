# OpenWA-Go

> **WhatsApp API Gateway** — Go implementation powered by [whatsmeow](https://github.com/tulir/whatsmeow), a pure-Go WhatsApp Web multi-device library.

OpenWA-Go is a reimplementation of [OpenWA](https://github.com/Aorus22/OpenWA) that replaces the Node.js WhatsApp engines (`whatsapp-web.js` / `Baileys`) with **whatsmeow**. The switch was motivated by persistent issues with the original engine layer: frequent session drops requiring QR re-scans, the overhead of running a full Chromium browser per session, and recurring authentication loops. whatsmeow solves all of that — no browser needed, sessions stay valid even after long inactivity, and the Go binary is lightweight with fast startup. The frontend dashboard is sourced from the original OpenWA project — full credit to [Yudhi Armyndharis & OpenWA Contributors](https://github.com/Aorus22/OpenWA) for the UI.

## Why Go + whatsmeow?

The original OpenWA is a great project, but the WhatsApp engine layer (Puppeteer/Chromium for `whatsapp-web.js`, WebSocket for `Baileys`) was a source of instability:

- **Session drops** — Saved sessions frequently required re-scanning the QR code after a few days of inactivity.
- **Chromium overhead** — `whatsapp-web.js` spins up a full browser per session, consuming excessive memory and CPU.
- **Stuck authentication** — The "authenticating" loop after QR scan was a recurring issue across setups.

**whatsmeow** eliminates all of that:

- **No browser** — Pure WebSocket protocol implementation. No Chromium, no Puppeteer, no headless shenanigans.
- **Persistent sessions** — Sessions survive weeks of inactivity. The credentials stored by whatsmeow remain valid long after the phone's main client is idle.
- **Lightweight** — A single Go binary vs Node.js + Chrome. Lower memory, faster startup.
- **Stable** — whatsmeow is battle-tested in production by the [Mautrix-WhatsApp](https://github.com/mautrix/whatsapp) bridge (serving thousands of Matrix-WhatsApp users daily).

## Architecture

```
┌─────────────┐    ┌──────────────────────┐    ┌──────────────────┐
│  Dashboard   │───▶│   REST API (Gin)     │───▶│  Session Manager  │
│  (React SPA) │    │   WebSocket (future) │    │  + Engine Factory │
└─────────────┘    └──────────────────────┘    └────────┬─────────┘
                                                        │
                                               ┌────────▼─────────┐
                                               │  whatsmeow       │
                                               │  (pure-Go, no    │
                                               │   browser)       │
                                               └────────┬─────────┘
                                                        │
                                               ┌────────▼─────────┐
                                               │  WhatsApp Servers │
                                               └──────────────────┘
```

### Database Layout

| Database | Contents | Scope |
|----------|----------|-------|
| `main.sqlite` | API keys, audit logs, settings | Shared (all sessions) |
| `openwa.sqlite` | Sessions, messages, webhooks, templates | Shared |
| `data/whatsmeow/*.db` | WhatsApp auth credentials | **Per session** |

## Quick Start

```bash
# 1. Clone
git clone https://github.com/Aorus22/OpenWA-Go.git
cd OpenWA-Go

# 2. Configure
cp .env.example .env
# Edit .env — set API_MASTER_KEY to a secure value

# 3. Build & run
docker compose up -d

# 4. Open dashboard
# http://localhost:2785
```

### Without Docker

```bash
go run ./cmd/server
```

## API Endpoints

### Sessions
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/sessions` | List all sessions |
| `POST` | `/api/sessions` | Create a session |
| `GET` | `/api/sessions/:id` | Get session details |
| `DELETE` | `/api/sessions/:id` | Delete a session |
| `POST` | `/api/sessions/:id/start` | Start a session |
| `POST` | `/api/sessions/:id/stop` | Stop a session |
| `POST` | `/api/sessions/:id/force-kill` | Force-kill a stuck session |
| `GET` | `/api/sessions/:id/qr` | Get QR code for pairing |
| `POST` | `/api/sessions/:id/pairing-code` | Request pairing code |

### Messaging
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/sessions/:id/messages/send-text` | Send text |
| `POST` | `/api/sessions/:id/messages/send-image` | Send image |
| `POST` | `/api/sessions/:id/messages/send-video` | Send video |
| `POST` | `/api/sessions/:id/messages/send-audio` | Send audio |
| `POST` | `/api/sessions/:id/messages/send-document` | Send document |
| `POST` | `/api/sessions/:id/messages/send-location` | Send location |
| `POST` | `/api/sessions/:id/messages/send-contact` | Send contact |
| `POST` | `/api/sessions/:id/messages/send-sticker` | Send sticker |
| `POST` | `/api/sessions/:id/messages/reply` | Reply to message |
| `POST` | `/api/sessions/:id/messages/forward` | Forward message |
| `POST` | `/api/sessions/:id/messages/react` | React to message |
| `POST` | `/api/sessions/:id/messages/delete` | Delete message |
| `GET` | `/api/sessions/:id/messages?chatId=...` | Get chat messages |

### Groups
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/sessions/:id/groups` | List groups |
| `GET` | `/api/sessions/:id/groups/:groupId` | Group info |
| `POST` | `/api/sessions/:id/groups` | Create group |
| `POST` | `/api/sessions/:id/groups/:id/participants` | Add participants |
| `DELETE` | `/api/sessions/:id/groups/:id/participants` | Remove participants |
| `POST` | `/api/sessions/:id/groups/:id/participants/promote` | Promote to admin |
| `POST` | `/api/sessions/:id/groups/:id/participants/demote` | Demote from admin |
| `POST` | `/api/sessions/:id/groups/:id/leave` | Leave group |
| `PUT` | `/api/sessions/:id/groups/:id/subject` | Set group name |
| `PUT` | `/api/sessions/:id/groups/:id/description` | Set group description |
| `GET` | `/api/sessions/:id/groups/:id/invite-code` | Get invite link |
| `POST` | `/api/sessions/:id/groups/:id/invite-code/revoke` | Revoke invite link |

### Contacts & Chats
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/sessions/:id/contacts` | List contacts |
| `GET` | `/api/sessions/:id/contacts/:contactId` | Get contact |
| `GET` | `/api/sessions/:id/contacts/check/:number` | Check if number is on WhatsApp |
| `GET` | `/api/sessions/:id/contacts/:id/profile-picture` | Get profile picture |
| `POST` | `/api/sessions/:id/contacts/:id/block` | Block contact |
| `DELETE` | `/api/sessions/:id/contacts/:id/block` | Unblock contact |
| `GET` | `/api/sessions/:id/chats` | List chats |
| `POST` | `/api/sessions/:id/chats/read` | Mark as read |
| `POST` | `/api/sessions/:id/chats/unread` | Mark as unread |
| `POST` | `/api/sessions/:id/chats/delete` | Delete chat |
| `POST` | `/api/sessions/:id/chats/typing` | Send typing indicator |

### Webhooks
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/webhooks` | List all webhooks |
| `GET` | `/api/sessions/:id/webhooks` | List session webhooks |
| `POST` | `/api/sessions/:id/webhooks` | Create webhook |
| `PUT` | `/api/sessions/:id/webhooks/:webhookId` | Update webhook |
| `DELETE` | `/api/sessions/:id/webhooks/:webhookId` | Delete webhook |

### Administration
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/auth/api-keys` | List API keys |
| `POST` | `/api/auth/api-keys` | Create API key |
| `GET` | `/api/auth/api-keys/:id` | Get API key |
| `PUT` | `/api/auth/api-keys/:id` | Update API key |
| `DELETE` | `/api/auth/api-keys/:id` | Delete API key |
| `POST` | `/api/auth/api-keys/:id/revoke` | Toggle API key |
| `POST` | `/api/auth/validate` | Validate API key |
| `GET` | `/api/audit` | List audit logs |
| `GET` | `/api/infra/status` | Infrastructure status |
| `GET` | `/api/infra/engines` | List engines |
| `GET` | `/api/sessions/stats/overview` | Session statistics |
| `GET` | `/api/stats/overview` | Overall statistics |
| `GET` | `/api/health` | Health check |

## Environment Variables

All configurable via `.env` file. See [.env.example](./.env.example) for the full list.

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `2785` | HTTP listen port |
| `DATABASE_TYPE` | `sqlite` | Database backend (`sqlite` / `postgres`) |
| `API_MASTER_KEY` | — | Master API key for admin access |
| `LOG_LEVEL` | `info` | Log verbosity |
| `AUTO_START_SESSIONS` | `false` | Auto-start authenticated sessions on boot |
| `SERVE_DASHBOARD` | `true` | Serve the dashboard SPA |
| `STORAGE_TYPE` | `local` | Media storage backend (`local` / `s3`) |

## Credits

- **[OpenWA](https://github.com/Aorus22/OpenWA)** — The original Node.js WhatsApp API Gateway. The frontend dashboard used in this project is sourced from OpenWA.
- **[whatsmeow](https://github.com/tulir/whatsmeow)** — Pure-Go WhatsApp Web multi-device library by [Tulir Asokan](https://github.com/tulir).
- **[Gin](https://github.com/gin-gonic/gin)** — HTTP web framework for Go.
- **[GORM](https://gorm.io)** — ORM library for Go.

## License

MIT
