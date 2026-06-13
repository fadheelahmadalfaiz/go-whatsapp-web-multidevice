package config

import (
	"time"

	"go.mau.fi/whatsmeow/proto/waCompanionReg"
)

var (
	AppVersion             = "v8.8.0"
	AppPort                = "3000"
	AppHost                = "0.0.0.0"
	AppDebug               = false
	AppOs                  = "GOWA"
	AppPlatform            = waCompanionReg.DeviceProps_PlatformType(1)
	AppBasicAuthCredential []string
	AppBasePath            = ""
	AppTrustedProxies      []string // Trusted proxy IP ranges (e.g., "0.0.0.0/0" for all, or specific CIDRs)

	McpPort = "8080"
	McpHost = "localhost"

	PathQrCode    = "statics/qrcode"
	PathSendItems = "statics/senditems"
	PathMedia     = "statics/media"
	PathStorages  = "storages"

	DBURI     = "file:storages/whatsapp.db"
	DBKeysURI = ""

	WhatsappAutoReplyMessage          string
	WhatsappAutoMarkRead              = false // Auto-mark incoming messages as read
	WhatsappAutoDownloadMedia         = true  // Auto-download media from incoming messages
	WhatsappWebhook                   []string
	WhatsappWebhookSecret             = "secret"
	WhatsappWebhookInsecureSkipVerify = false          // Skip TLS certificate verification for webhooks (insecure)
	WhatsappWebhookEvents             []string         // Whitelist of events to forward to webhook (empty = all events)
	WhatsappAutoRejectCall                     = false // Auto-reject incoming calls
	WhatsappLogLevel                           = "ERROR"
	WhatsappSettingMaxImageSize       int64    = 20000000  // 20MB
	WhatsappSettingMaxFileSize        int64    = 50000000  // 50MB
	WhatsappSettingMaxVideoSize       int64    = 100000000 // 100MB
	WhatsappSettingMaxDownloadSize    int64    = 500000000 // 500MB
	WhatsappTypeUser                           = "@s.whatsapp.net"
	WhatsappTypeGroup                          = "@g.us"
	WhatsappTypeLid                            = "@lid"
	WhatsappAccountValidation                  = true
	WhatsappPresenceOnConnect                  = "unavailable" // Presence to send on connect: "available", "unavailable", or "none"
	WhatsappPresencePulseEnabled               = true          // Periodically pulse presence available, then unavailable
	WhatsappPresencePulseInterval              = 24 * time.Hour
	WhatsappPresencePulseDuration              = 5 * time.Minute

	ChatStorageURI               = "file:storages/chatstorage.db"
	ChatStorageEnableForeignKeys = true
	ChatStorageEnableWAL         = true

	ChatwootEnabled   = false
	ChatwootURL       = ""
	ChatwootAPIToken  = ""
	ChatwootAccountID = 0
	ChatwootInboxID   = 0
	ChatwootDeviceID  = "" // Device ID for outbound messages (required for multi-device)

	// Chatwoot History Sync settings
	ChatwootImportMessages          = false // Enable message history import to Chatwoot
	ChatwootDaysLimitImportMessages = 3     // Days of history to import (default: 3)

	// ChatwootImportDBURI, when set, enables the direct-Postgres import path.
	ChatwootImportDBURI = ""

	// ChatwootImportPlaceholderMediaMessage controls what is inserted as the
	// message body for media messages when the importer could not download
	// the media file (e.g., URL expired).
	ChatwootImportPlaceholderMediaMessage = true
	// ChatwootImportMediaWithREST sends media history rows through Chatwoot's
	// REST attachment endpoint while direct-DB import handles non-media rows.
	ChatwootImportMediaWithREST = false

	// Chatwoot auto-provisioning
	ChatwootAutoCreate = false
	ChatwootInboxName  = "WhatsApp"
	ChatwootWebhookURL = ""
	// Optional shared secret for incoming Chatwoot webhooks.
	ChatwootWebhookSecret = ""

	// Chatwoot conversation handling
	ChatwootReopenConversation  = true
	ChatwootConversationPending = false

	// ChatwootIgnoreJids lists WhatsApp JIDs that must never be mirrored to Chatwoot.
	ChatwootIgnoreJids []string

	// Chatwoot outbound signature
	ChatwootSignMsg       = false
	ChatwootSignDelimiter = "\n\n"

	// Chatwoot edit/delete propagation
	ChatwootForwardEdits   = true
	ChatwootForwardDeletes = true

	// Chatwoot Evolution-compatible state propagation
	ChatwootMessageRead   = false
	ChatwootMessageDelete = false

	// S3-compatible object storage configuration
	S3 = S3Config{
		Region: "auto",
	}
)
