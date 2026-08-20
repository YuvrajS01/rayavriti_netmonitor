package handlers

import (
	"strings"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// secretConfigKeys are the keys in a notification channel's Config map that
// contain sensitive values (passwords, tokens, webhook URLs with embedded
// secrets, API keys). These are masked in List/Get API responses.
var secretConfigKeys = map[string]bool{
	"password":      true,
	"token":         true,
	"api_key":       true,
	"apikey":        true,
	"webhook_url":   true,
	"webhookurl":    true,
	"url":           true,
	"secret":        true,
	"auth_token":    true,
	"client_secret": true,
	"access_token":  true,
	"refresh_token": true,
	"smtp_password": true,
	"private_key":   true,
	"passphrase":    true,
}

// maskValue replaces a secret string with a masked placeholder, preserving
// whether a value was set. The first and last characters are shown when the
// value is long enough, otherwise it's fully masked.
func maskValue(v string) string {
	if len(v) <= 4 {
		return "****"
	}
	return v[:1] + strings.Repeat("*", len(v)-2) + v[len(v)-1:]
}

// maskChannelConfig returns a shallow copy of the config map with secret
// values masked. Non-secret keys are passed through unchanged.
func maskChannelConfig(cfg map[string]any) map[string]any {
	if cfg == nil {
		return nil
	}
	masked := make(map[string]any, len(cfg))
	for k, v := range cfg {
		if secretConfigKeys[strings.ToLower(k)] {
			switch val := v.(type) {
			case string:
				if val != "" {
					masked[k] = maskValue(val)
				} else {
					masked[k] = val
				}
			default:
				// Non-string secrets (rare): mask generically.
				masked[k] = "****"
			}
		} else {
			masked[k] = v
		}
	}
	return masked
}

// maskChannel returns a shallow copy of a NotificationChannel with its Config
// secrets masked. The original channel is not modified.
func maskChannel(ch *models.NotificationChannel) *models.NotificationChannel {
	if ch == nil {
		return nil
	}
	cp := *ch
	cp.Config = maskChannelConfig(ch.Config)
	return &cp
}

// maskChannels returns a slice with all channel configs masked.
func maskChannels(channels []models.NotificationChannel) []models.NotificationChannel {
	for i := range channels {
		channels[i].Config = maskChannelConfig(channels[i].Config)
	}
	return channels
}
