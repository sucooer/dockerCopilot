package utiles

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/onlyLTY/dockerCopilot/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

const notifyConfigFile = "notification.json"

func notifyConfigPath() string {
	return filepath.Join(configDir, notifyConfigFile)
}

func LoadNotifyConfig() (*types.NotifyConfig, error) {
	path := notifyConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &types.NotifyConfig{Channels: []types.NotifyChannel{}}, nil
		}
		return nil, fmt.Errorf("failed to read notify config: %w", err)
	}
	var cfg types.NotifyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse notify config: %w", err)
	}
	return &cfg, nil
}

func SaveNotifyConfig(cfg *types.NotifyConfig) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal notify config: %w", err)
	}
	path := notifyConfigPath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write notify config: %w", err)
	}
	return nil
}

func SendNotify(containerName, image string, success bool, errMsg string) {
	cfg, err := LoadNotifyConfig()
	if err != nil {
		logx.Errorf("notify: failed to load config: %v", err)
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	var title, msg string

	if success {
		title = "✅ Docker Copilot - 自动更新成功"
		msg = fmt.Sprintf("容器: %s\n镜像: %s\n时间: %s", containerName, image, now)
	} else {
		title = "❌ Docker Copilot - 自动更新失败"
		msg = fmt.Sprintf("容器: %s\n镜像: %s\n错误: %s\n时间: %s", containerName, image, errMsg, now)
	}

	for _, ch := range cfg.Channels {
		if !ch.Enabled {
			continue
		}
		switch ch.Type {
		case types.NotifyTelegram:
			sendTelegram(ch, title, msg)
		case types.NotifyServerChan:
			sendServerChan(ch, title, msg)
		case types.NotifyWebhook:
			sendWebhook(ch, title, msg)
		}
	}
}

func TestNotify(channel types.NotifyChannel) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	title := "🔔 Docker Copilot - 通知测试"
	msg := fmt.Sprintf("通知通道配置正常\n当前时间: %s", now)

	switch channel.Type {
	case types.NotifyTelegram:
		return sendTelegram(channel, title, msg)
	case types.NotifyServerChan:
		return sendServerChan(channel, title, msg)
	case types.NotifyWebhook:
		return sendWebhook(channel, title, msg)
	default:
		return fmt.Errorf("unknown channel type: %s", channel.Type)
	}
}

func sendTelegram(ch types.NotifyChannel, title, msg string) error {
	if ch.BotToken == "" || ch.ChatID == "" {
		return fmt.Errorf("telegram: botToken or chatId is empty")
	}
	text := fmt.Sprintf("%s\n\n%s", title, msg)
	body := map[string]interface{}{
		"chat_id":    ch.ChatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	payload, _ := json.Marshal(body)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", ch.BotToken)
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: status %d", resp.StatusCode)
	}
	return nil
}

func sendServerChan(ch types.NotifyChannel, title, msg string) error {
	if ch.SendKey == "" {
		return fmt.Errorf("serverchan: sendKey is empty")
	}
	body := map[string]string{
		"title": "Docker Copilot",
		"desp":  fmt.Sprintf("%s\n\n%s", title, msg),
	}
	payload, _ := json.Marshal(body)
	url := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", ch.SendKey)
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("serverchan: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("serverchan: status %d", resp.StatusCode)
	}
	return nil
}

func sendWebhook(ch types.NotifyChannel, title, msg string) error {
	if ch.WebhookURL == "" {
		return fmt.Errorf("webhook: url is empty")
	}
	body := map[string]string{
		"title":   title,
		"content": msg,
	}
	payload, _ := json.Marshal(body)
	resp, err := http.Post(ch.WebhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: status %d", resp.StatusCode)
	}
	return nil
}
