package campus

import (
	"context"
	"fmt"
	"net"
	"time"
)

// MaintenanceWindow represents a scheduled maintenance period.
type MaintenanceWindow struct {
	ID                    int64   `json:"id"`
	Name                  string  `json:"name"`
	Description           string  `json:"description,omitempty"`
	ScopeType             string  `json:"scopeType"`
	ScopeValue            string  `json:"scopeValue,omitempty"`
	ScheduleType          string  `json:"scheduleType"`
	StartTime             *string `json:"startTime,omitempty"`
	EndTime               *string `json:"endTime,omitempty"`
	RecurrenceRule        *string `json:"recurrenceRule,omitempty"`
	RecurrenceStartTime   *string `json:"recurrenceStartTime,omitempty"`
	RecurrenceEndTime     *string `json:"recurrenceEndTime,omitempty"`
	RecurrenceTimezone    string  `json:"recurrenceTimezone,omitempty"`
	SuppressAlerts        bool    `json:"suppressAlerts"`
	SuppressNotifications bool    `json:"suppressNotifications"`
	ShowMaintenanceStatus bool    `json:"showMaintenanceStatus"`
	CreatedBy             string  `json:"createdBy,omitempty"`
	Enabled               bool    `json:"enabled"`
}

// MaintenanceService evaluates whether a device is currently in a maintenance window.
type MaintenanceService struct {
	db DB
}

// NewMaintenanceService creates a new MaintenanceService.
func NewMaintenanceService(db DB) *MaintenanceService {
	return &MaintenanceService{db: db}
}

// MaintenanceStatus indicates whether a device is under maintenance and why.
type MaintenanceStatus struct {
	UnderMaintenance bool               `json:"underMaintenance"`
	SuppressAlerts   bool               `json:"suppressAlerts"`
	SuppressNotify   bool               `json:"suppressNotifications"`
	Window           *MaintenanceWindow `json:"activeWindow,omitempty"`
}

// IsUnderMaintenance checks whether the given device is currently within any
// active maintenance window that applies to it. The deviceID, locationID, and
// deviceIP are used to match against scope_type/scope_value.
func (s *MaintenanceService) IsUnderMaintenance(ctx context.Context, deviceID int64, locationID *int64, deviceIP string) (*MaintenanceStatus, error) {
	windows, err := s.getActiveWindows(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	for _, w := range windows {
		if !w.Enabled {
			continue
		}
		if !s.windowAppliesTo(w, deviceID, locationID, deviceIP) {
			continue
		}
		if !s.windowIsActive(w, now) {
			continue
		}
		return &MaintenanceStatus{
			UnderMaintenance: true,
			SuppressAlerts:   w.SuppressAlerts,
			SuppressNotify:   w.SuppressNotifications,
			Window:           w,
		}, nil
	}

	return &MaintenanceStatus{UnderMaintenance: false}, nil
}

func (s *MaintenanceService) getActiveWindows(ctx context.Context) ([]*MaintenanceWindow, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, name, COALESCE(description,''), scope_type, COALESCE(scope_value,''),
			schedule_type, start_time, end_time, recurrence_rule,
			recurrence_start_time, recurrence_end_time, COALESCE(recurrence_timezone,''),
			suppress_alerts, suppress_notifications, show_maintenance_status,
			COALESCE(created_by,''), enabled
		FROM maintenance_windows WHERE enabled=TRUE ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query maintenance windows: %w", err)
	}
	defer rows.Close()

	var windows []*MaintenanceWindow
	for rows.Next() {
		var w MaintenanceWindow
		if err := rows.Scan(
			&w.ID, &w.Name, &w.Description, &w.ScopeType, &w.ScopeValue,
			&w.ScheduleType, &w.StartTime, &w.EndTime, &w.RecurrenceRule,
			&w.RecurrenceStartTime, &w.RecurrenceEndTime, &w.RecurrenceTimezone,
			&w.SuppressAlerts, &w.SuppressNotifications, &w.ShowMaintenanceStatus,
			&w.CreatedBy, &w.Enabled,
		); err != nil {
			return nil, err
		}
		windows = append(windows, &w)
	}
	return windows, rows.Err()
}

// windowAppliesTo checks whether a maintenance window matches the given device.
func (s *MaintenanceService) windowAppliesTo(w *MaintenanceWindow, deviceID int64, locationID *int64, deviceIP string) bool {
	switch w.ScopeType {
	case "global":
		return true
	case "device":
		return w.ScopeValue == fmt.Sprintf("%d", deviceID)
	case "location":
		if locationID == nil {
			return false
		}
		return w.ScopeValue == fmt.Sprintf("%d", *locationID)
	case "subnet":
		return matchesSubnet(deviceIP, w.ScopeValue)
	default:
		return false
	}
}

// windowIsActive checks whether a maintenance window is currently active
// based on its schedule type (once or recurring).
func (s *MaintenanceService) windowIsActive(w *MaintenanceWindow, now time.Time) bool {
	switch w.ScheduleType {
	case "once":
		return s.isActiveOnce(w, now)
	case "recurring":
		return s.isActiveRecurring(w, now)
	default:
		return false
	}
}

func (s *MaintenanceService) isActiveOnce(w *MaintenanceWindow, now time.Time) bool {
	if w.StartTime == nil || w.EndTime == nil {
		return false
	}
	start, err1 := time.Parse(time.RFC3339, *w.StartTime)
	end, err2 := time.Parse(time.RFC3339, *w.EndTime)
	if err1 != nil || err2 != nil {
		return false
	}
	return now.After(start) && now.Before(end)
}

func (s *MaintenanceService) isActiveRecurring(w *MaintenanceWindow, now time.Time) bool {
	if w.RecurrenceRule == nil || w.RecurrenceStartTime == nil || w.RecurrenceEndTime == nil {
		return false
	}
	recStart, err1 := time.Parse(time.RFC3339, *w.RecurrenceStartTime)
	recEnd, err2 := time.Parse(time.RFC3339, *w.RecurrenceEndTime)
	if err1 != nil || err2 != nil {
		return false
	}

	if now.Before(recStart) || now.After(recEnd) {
		return false
	}

	if w.StartTime == nil || w.EndTime == nil {
		return false
	}
	windowStart, err1 := time.Parse(time.RFC3339, *w.StartTime)
	windowEnd, err2 := time.Parse(time.RFC3339, *w.EndTime)
	if err1 != nil || err2 != nil {
		return false
	}

	duration := windowEnd.Sub(windowStart)
	dailyStart := time.Date(now.Year(), now.Month(), now.Day(),
		windowStart.Hour(), windowStart.Minute(), windowStart.Second(), 0, now.Location())
	dailyEnd := dailyStart.Add(duration)

	return now.After(dailyStart) && now.Before(dailyEnd)
}

// matchesSubnet checks if an IP is within a CIDR subnet using the net package.
func matchesSubnet(ip, cidr string) bool {
	if cidr == "" || ip == "" {
		return false
	}
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	return subnet.Contains(parsedIP)
}
