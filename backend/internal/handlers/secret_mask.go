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
	"bot_token":     true, // Telegram
	"headers":       true, // webhook headers usually carry Authorization
	"community":     true, // SNMP
}

// secretPlaceholder is what clients see in place of a stored secret. A
// client that sends it back on update keeps the stored value.
const secretPlaceholder = "********"

// maskValue replaces a secret with a fixed placeholder that reveals only
// whether a value is set (no characters or length).
func maskValue(string) string {
	return secretPlaceholder
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
				// Non-string secrets, e.g. a headers map.
				masked[k] = secretPlaceholder
			}
		} else {
			masked[k] = v
		}
	}
	return masked
}

// restoreMaskedSecrets copies stored secrets into an incoming config where
// the client sent the placeholder back unchanged, so re-saving a channel from
// a form that displays masked values doesn't overwrite the real secret.
func restoreMaskedSecrets(incoming, stored map[string]any) {
	for k, v := range incoming {
		if s, ok := v.(string); ok && s == secretPlaceholder && secretConfigKeys[strings.ToLower(k)] {
			if orig, ok := stored[k]; ok {
				incoming[k] = orig
			} else {
				delete(incoming, k)
			}
		}
	}
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
