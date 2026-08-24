package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIncidentHandler_NilPool(t *testing.T) {
	h := &IncidentHandler{}

	tests := []struct {
		name   string
		method func(http.ResponseWriter, *http.Request)
	}{
		{"CreateIncident", h.CreateIncident},
		{"AcknowledgeIncident", h.AcknowledgeIncident},
		{"ResolveIncident", h.ResolveIncident},
		{"CloseIncident", h.CloseIncident},
		{"AssignIncident", h.AssignIncident},
		{"AddTimelineEntry", h.AddTimelineEntry},
		{"GetTimeline", h.GetTimeline},
		{"GetIncidentDevices", h.GetIncidentDevices},
		{"GetIncidentStats", h.GetIncidentStats},
		{"GetSLAReport", h.GetSLAReport},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			tt.method(w, req)
			if w.Code != http.StatusNotImplemented {
				t.Errorf("expected 501, got %d", w.Code)
			}
		})
	}
}

func TestStatusPageHandler_NilPool(t *testing.T) {
	h := &StatusPageHandler{}

	tests := []struct {
		name   string
		method func(http.ResponseWriter, *http.Request)
	}{
		{"PublicStatusJSON", h.PublicStatusJSON},
		{"AddServiceDevice", h.AddServiceDevice},
		{"RemoveServiceDevice", h.RemoveServiceDevice},
		{"ListServiceDevices", h.ListServiceDevices},
		{"LinkIncidentServices", h.LinkIncidentServices},
		{"ListIncidentUpdates", h.ListIncidentUpdates},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			tt.method(w, req)
			if w.Code != http.StatusNotImplemented {
				t.Errorf("expected 501, got %d", w.Code)
			}
		})
	}
}

func TestISPHandler_NilPool(t *testing.T) {
	h := &ISPHandler{}

	tests := []struct {
		name   string
		method func(http.ResponseWriter, *http.Request)
	}{
		{"Comparison", h.Comparison},
		{"LinkSLA", h.LinkSLA},
		{"MetricsSummary", h.MetricsSummary},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			tt.method(w, req)
			if w.Code != http.StatusNotImplemented {
				t.Errorf("expected 501, got %d", w.Code)
			}
		})
	}
}

func TestNewIncidentHandler_NonPostgres(t *testing.T) {
	h := NewIncidentHandler(&mockDB{}, nil)
	if h.pool != nil {
		t.Error("expected nil pool for non-Postgres db")
	}
}

func TestNewStatusPageHandler_NonPostgres(t *testing.T) {
	h := NewStatusPageHandler(&mockDB{})
	if h.pool != nil {
		t.Error("expected nil pool for non-Postgres db")
	}
}

func TestNewISPHandler_NonPostgres(t *testing.T) {
	h := NewISPHandler(&mockDB{})
	if h.pool != nil {
		t.Error("expected nil pool for non-Postgres db")
	}
}
