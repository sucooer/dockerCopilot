package config

import (
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	ComposeDir     string
	ImageDir       string
	ImageLogosPath string
	AllowedOrigins []string
	Auth           struct { // JWT 认证需要的密钥和过期时间配置
		AccessSecret string
		AccessExpire int64
	}
}

var (
	Version   string
	BuildDate string
)

var weakSecretKeys = map[string]bool{
	"dockercopilot":     true,
	"dockercopilot2024": true,
	"dockerCopilot":     true,
	"dockerCopilot2024": true,
	"dockercopilot123":  true,
	"123456789":         true,
	"1234567890":        true,
	"password123":       true,
	"qwerty12345":       true,
	"admin12345":        true,
}

// ValidateSecretKey 校验 secretKey 是否满足安全要求：
// 大于 8 位、非纯数字、非常见弱口令、非未替换的环境变量占位符。
func ValidateSecretKey(secret string) error {
	if secret == "" {
		return fmt.Errorf("secretKey 不能为空")
	}
	if strings.Contains(secret, "${") {
		return fmt.Errorf("secretKey 未正确配置（检测到未替换的环境变量占位符）")
	}
	if len(secret) < 9 {
		return fmt.Errorf("secretKey 长度必须大于 8 位")
	}
	if weakSecretKeys[secret] {
		return fmt.Errorf("secretKey 为常见弱口令，请更换为随机强密钥")
	}
	if isAllDigits(secret) {
		return fmt.Errorf("secretKey 不能为纯数字")
	}
	return nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
