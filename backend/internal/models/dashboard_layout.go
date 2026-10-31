package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// Dashboard layout limits. A layout is a 12-column grid of widgets in
// reading order; the client lays them out with CSS grid spans.
const (
	DashboardLayoutVersion   = 1
	DashboardMaxWidgets      = 48
	DashboardMaxLayoutBytes  = 64 << 10
	DashboardMaxConfigBytes  = 4 << 10
	DashboardGridColumns     = 12
	DashboardMaxWidgetHeight = 4
	DashboardMaxNameLength   = 100
)

// EmptyDashboardLayout is stored when a dashboard is created without one.
var EmptyDashboardLayout = json.RawMessage(`{"version":1,"widgets":[]}`)

// DashboardWidget is one widget in a layout. Type names a client widget;
// the server only checks its shape, so new widget types need no backend
// change. Config is widget-specific and opaque here.
type DashboardWidget struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	W      int             `json:"w"`
	H      int             `json:"h"`
	Config json.RawMessage `json:"config,omitempty"`
}

type DashboardLayout struct {
	Version int               `json:"version"`
	Widgets []DashboardWidget `json:"widgets"`
}

var (
	widgetIDPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	widgetTypePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,47}$`)
)

// ValidateDashboardLayout checks a layout's structure and size and returns
// it re-encoded, so unknown top-level fields are not stored.
func ValidateDashboardLayout(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) > DashboardMaxLayoutBytes {
		return nil, fmt.Errorf("layout must be at most %d KB", DashboardMaxLayoutBytes>>10)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var l DashboardLayout
	if err := dec.Decode(&l); err != nil {
		return nil, errors.New("layout must be an object with version and widgets")
	}
	if l.Version != DashboardLayoutVersion {
		return nil, fmt.Errorf("layout version must be %d", DashboardLayoutVersion)
	}
	if len(l.Widgets) > DashboardMaxWidgets {
		return nil, fmt.Errorf("a dashboard can have at most %d widgets", DashboardMaxWidgets)
	}
	seen := make(map[string]bool, len(l.Widgets))
	for i, w := range l.Widgets {
		if !widgetIDPattern.MatchString(w.ID) {
			return nil, fmt.Errorf("widget %d: id must be 1-64 letters, digits, _ or -", i)
		}
		if seen[w.ID] {
			return nil, fmt.Errorf("widget %d: duplicate id %q", i, w.ID)
		}
		seen[w.ID] = true
		if !widgetTypePattern.MatchString(w.Type) {
			return nil, fmt.Errorf("widget %q: invalid type", w.ID)
		}
		if w.W < 1 || w.W > DashboardGridColumns {
			return nil, fmt.Errorf("widget %q: w must be 1-%d", w.ID, DashboardGridColumns)
		}
		if w.H < 1 || w.H > DashboardMaxWidgetHeight {
			return nil, fmt.Errorf("widget %q: h must be 1-%d", w.ID, DashboardMaxWidgetHeight)
		}
		if len(w.Config) > 0 {
			if len(w.Config) > DashboardMaxConfigBytes {
				return nil, fmt.Errorf("widget %q: config must be at most %d KB", w.ID, DashboardMaxConfigBytes>>10)
			}
			if c := bytes.TrimSpace(w.Config); len(c) == 0 || (c[0] != '{' && string(c) != "null") {
				return nil, fmt.Errorf("widget %q: config must be an object", w.ID)
			}
		}
	}
	if l.Widgets == nil {
		l.Widgets = []DashboardWidget{}
	}
	return json.Marshal(l)
}
