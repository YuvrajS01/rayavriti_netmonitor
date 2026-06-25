package engine

import (
	"context"
	"log/slog"

	"github.com/rayavriti/netmonitor-backend/internal/campus"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// SuppressionChecker determines whether an alert for a device should be
// suppressed based on topology (parent down) or maintenance windows.
type SuppressionChecker interface {
	CheckSuppression(ctx context.Context, deviceID int64) (*campus.SuppressionResult, error)
}

// MaintenanceChecker determines whether a device is currently in a
// maintenance window that suppresses alerts.
type MaintenanceChecker interface {
	IsUnderMaintenance(ctx context.Context, deviceID int64, locationID *int64, deviceIP string) (*campus.MaintenanceStatus, error)
}

// SuppressedAlertRecorder persists a record of a suppressed alert.
type SuppressedAlertRecorder interface {
	RecordSuppressedAlert(ctx context.Context, deviceID int64, ruleID *int64, reason string, rootCauseDeviceID *int64) error
}

// checkSuppressionForDevice evaluates all suppression sources for a device and
// returns true if the alert should be suppressed. Logs the reason.
func (e *AlertEngine) checkSuppressionForDevice(
	ctx context.Context,
	device *models.Device,
	rule *models.AlertRule,
) bool {
	// 1. Topology-based suppression (parent device is down)
	if e.suppressionChecker != nil {
		result, err := e.suppressionChecker.CheckSuppression(ctx, device.ID)
		if err != nil {
			slog.Warn("Suppression check failed",
				"device_id", device.ID, "error", err)
		} else if result.ShouldSuppress {
			slog.Info("Alert suppressed by topology",
				"device_id", device.ID, "rule_id", rule.ID,
				"reason", result.Reason,
				"root_cause", result.RootCauseDevice)

			var rootCauseID *int64
			if result.RootCauseDevice != nil {
				rootCauseID = &result.RootCauseDevice.DeviceID
			}
			e.recordSuppressedAlert(ctx, device.ID, &rule.ID, result.Reason, rootCauseID)
			return true
		}
	}

	// 2. Maintenance window suppression
	if e.maintenanceChecker != nil {
		var locID *int64
		if device.LocationID != nil {
			locID = device.LocationID
		}
		status, err := e.maintenanceChecker.IsUnderMaintenance(ctx, device.ID, locID, device.IPAddress)
		if err != nil {
			slog.Warn("Maintenance check failed",
				"device_id", device.ID, "error", err)
		} else if status.UnderMaintenance && status.SuppressAlerts {
			slog.Info("Alert suppressed by maintenance window",
				"device_id", device.ID, "rule_id", rule.ID,
				"window", status.Window)

			e.recordSuppressedAlert(ctx, device.ID, &rule.ID, "maintenance_window", nil)
			return true
		}
	}

	return false
}

func (e *AlertEngine) recordSuppressedAlert(
	ctx context.Context,
	deviceID int64,
	ruleID *int64,
	reason string,
	rootCauseDeviceID *int64,
) {
	if e.suppressedRecorder == nil {
		return
	}
	if err := e.suppressedRecorder.RecordSuppressedAlert(ctx, deviceID, ruleID, reason, rootCauseDeviceID); err != nil {
		slog.Warn("Failed to record suppressed alert",
			"device_id", deviceID, "reason", reason, "error", err)
	}
}
