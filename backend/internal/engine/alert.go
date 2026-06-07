package engine

import (
	"context"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type AlertEngine struct {
	db database.Database
}

func NewAlertEngine(db database.Database) *AlertEngine {
	return &AlertEngine{db: db}
}

func (e *AlertEngine) EvaluateMetric(ctx context.Context, metric *models.Metric) error {
	// TODO: load alert rules from DB and evaluate conditions
	// For now, simple device_down check
	if metric.Status == "down" {
		device, err := e.db.GetDevice(ctx, metric.DeviceID)
		if err != nil {
			return err
		}
		// Check if alert already exists
		alerts, _, _ := e.db.GetAlerts(ctx, "active", 100, 0)
		for _, a := range alerts {
			if a.DeviceID == metric.DeviceID && a.Status == "active" {
				return nil // already alerted
			}
		}
		_, err = e.db.CreateAlert(ctx, &models.Alert{
			DeviceID:   metric.DeviceID,
			DeviceName: device.Name,
			Severity:   "critical",
			Message:    "Device " + device.Name + " is DOWN",
			Status:     "active",
		})
		return err
	}
	return nil
}
