package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// ContactResolver finds which contacts should be notified for a device/alert.
type ContactResolver struct {
	pool *pgxpool.Pool
	db   database.Database
}

// NewContactResolver creates a new ContactResolver.
func NewContactResolver(pool *pgxpool.Pool, db database.Database) *ContactResolver {
	return &ContactResolver{pool: pool, db: db}
}

// ResolvedContact is a contact with their notification target resolved.
type ResolvedContact struct {
	Contact    models.Contact
	Channel    string // "telegram", "email", "whatsapp", "sms"
	Target     string // chat ID, email, phone number
	Role       string // "primary", "secondary", "escalation"
	DeviceID   *int64
	LocationID *int64
}

// ResolveForDevice returns the contacts that should be notified for an alert
// on the given device, filtered by severity.
func (r *ContactResolver) ResolveForDevice(ctx context.Context, deviceID int64, locationID *int64, severity string) ([]ResolvedContact, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.id, c.name, COALESCE(c.email,''), COALESCE(c.phone,''),
			COALESCE(c.telegram_chat_id,''), COALESCE(c.whatsapp_number,''),
			COALESCE(c.preferred_channel,'email'), c.notification_enabled,
			c.quiet_hours_start, c.quiet_hours_end, dc.role, dc.notify_on,
			dc.device_id, dc.location_id
		FROM device_contacts dc
		JOIN contacts c ON c.id = dc.contact_id
		WHERE (dc.device_id = $1 OR dc.location_id = $2)
		  AND c.enabled = TRUE AND c.notification_enabled = TRUE
		ORDER BY dc.role, c.name`,
		deviceID, locationIDOrZero(locationID),
	)
	if err != nil {
		return nil, fmt.Errorf("query device contacts: %w", err)
	}
	defer rows.Close()

	var contacts []ResolvedContact
	for rows.Next() {
		var c models.Contact
		var role, notifyOn string
		var devID, locID *int64
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Email, &c.Phone,
			&c.TelegramChatID, &c.WhatsAppNumber,
			&c.PreferredChannel, &c.NotificationEnabled,
			&c.QuietHoursStart, &c.QuietHoursEnd,
			&role, &notifyOn,
			&devID, &locID,
		); err != nil {
			return nil, err
		}

		if !shouldNotify(notifyOn, severity) {
			continue
		}
		if inQuietHours(c.QuietHoursStart, c.QuietHoursEnd) {
			slog.Debug("Skipping contact in quiet hours", "contact", c.Name)
			continue
		}

		target := resolveTarget(c)
		if target == "" {
			continue
		}

		contacts = append(contacts, ResolvedContact{
			Contact:    c,
			Channel:    c.PreferredChannel,
			Target:     target,
			Role:       role,
			DeviceID:   devID,
			LocationID: locID,
		})
	}
	return contacts, rows.Err()
}

// ResolveForAlert resolves contacts from an alert's device.
func (r *ContactResolver) ResolveForAlert(ctx context.Context, alert *models.Alert, severity string) ([]ResolvedContact, error) {
	device, err := r.db.GetDevice(ctx, alert.DeviceID)
	if err != nil {
		return nil, err
	}
	return r.ResolveForDevice(ctx, device.ID, device.LocationID, severity)
}

func resolveTarget(c models.Contact) string {
	switch c.PreferredChannel {
	case "telegram":
		if c.TelegramChatID != "" {
			return c.TelegramChatID
		}
	case "whatsapp":
		if c.WhatsAppNumber != "" {
			return c.WhatsAppNumber
		}
	case "sms":
		if c.Phone != "" {
			return c.Phone
		}
	case "email":
		if c.Email != "" {
			return c.Email
		}
	}
	if c.TelegramChatID != "" {
		return c.TelegramChatID
	}
	if c.Email != "" {
		return c.Email
	}
	if c.Phone != "" {
		return c.Phone
	}
	if c.WhatsAppNumber != "" {
		return c.WhatsAppNumber
	}
	return ""
}

func shouldNotify(notifyOn, severity string) bool {
	if notifyOn == "" {
		return true
	}
	for _, s := range splitComma(notifyOn) {
		if s == severity {
			return true
		}
	}
	return false
}

func splitComma(s string) []string {
	var parts []string
	for _, p := range splitBytes(s, ',') {
		trimmed := trimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitBytes(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && s[start] == ' ' {
		start++
	}
	for end > start && s[end-1] == ' ' {
		end--
	}
	return s[start:end]
}

func inQuietHours(start, end *string) bool {
	if start == nil || end == nil {
		return false
	}
	now := time.Now()
	nowMinutes := now.Hour()*60 + now.Minute()

	sMinutes := parseHHMM(*start)
	eMinutes := parseHHMM(*end)
	if sMinutes < 0 || eMinutes < 0 {
		return false
	}

	if sMinutes <= eMinutes {
		return nowMinutes >= sMinutes && nowMinutes < eMinutes
	}
	return nowMinutes >= sMinutes || nowMinutes < eMinutes
}

func parseHHMM(s string) int {
	if len(s) < 5 || s[2] != ':' {
		return -1
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}

func locationIDOrZero(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}
