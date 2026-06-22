package types

type NotifyChannelType string

const (
	NotifyTelegram   NotifyChannelType = "telegram"
	NotifyServerChan NotifyChannelType = "serverchan"
	NotifyWebhook    NotifyChannelType = "webhook"
)

type NotifyConfig struct {
	Channels []NotifyChannel `json:"channels"`
}

type NotifyChannel struct {
	Type       NotifyChannelType `json:"type"`
	Enabled    bool              `json:"enabled"`
	BotToken   string            `json:"botToken,omitempty"`
	ChatID     string            `json:"chatId,omitempty"`
	SendKey    string            `json:"sendKey,omitempty"`
	WebhookURL string            `json:"webhookUrl,omitempty"`
}

type NotifyTestReq struct {
	Channel NotifyChannel `json:"channel"`
}
