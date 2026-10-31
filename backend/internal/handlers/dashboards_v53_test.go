package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

const otherUserID = testUserID + 100

func dashboardAs(r *http.Request, role string, perms ...string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), auth.ClaimsKey, &auth.Claims{UserID: testUserID, Role: role, Permissions: perms}))
}

func TestDashboard_SharedIsReadableButNotWritable(t *testing.T) {
	var saved, deleted bool
	db := &mockDB{
		getDashboardFn: func(ctx context.Context, id int64) (*models.Dashboard, error) {
			return &models.Dashboard{ID: id, UserID: otherUserID, Name: "NOC", Shared: id == 1, Layout: models.EmptyDashboardLayout}, nil
		},
		saveDashboardFn: func(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
			saved = true
			return d, nil
		},
		deleteDashboardFn: func(ctx context.Context, id int64) error {
			deleted = true
			return nil
		},
	}
	h := NewDashboardHandler(db)

	w, req := makeRequestWithParams("GET", "/api/v1/dashboards/1", "", "id", "1")
	h.Get(w, dashboardAs(req, "viewer", models.PermDevicesRead))
	if w.Code != http.StatusOK {
		t.Fatalf("shared dashboard: expected 200, got %d", w.Code)
	}
	w, req = makeRequestWithParams("GET", "/api/v1/dashboards/2", "", "id", "2")
	h.Get(w, dashboardAs(req, "viewer", models.PermDevicesRead))
	if w.Code != http.StatusNotFound {
		t.Fatalf("private dashboard of another user: expected 404, got %d", w.Code)
	}
	w, req = makeRequestWithParams("PUT", "/api/v1/dashboards/1", `{"name":"mine now"}`, "id", "1")
	h.Save(w, dashboardAs(req, "admin", models.PermDevicesWrite))
	if w.Code != http.StatusForbidden || saved {
		t.Fatalf("editing someone else's shared dashboard: expected 403, got %d (saved=%v)", w.Code, saved)
	}
	w, req = makeRequestWithParams("DELETE", "/api/v1/dashboards/1", "", "id", "1")
	h.Delete(w, dashboardAs(req, "admin", models.PermDevicesWrite))
	if w.Code != http.StatusForbidden || deleted {
		t.Fatalf("deleting someone else's shared dashboard: expected 403, got %d", w.Code)
	}
}

func TestDashboard_UpdateKeepsOmittedFields(t *testing.T) {
	layout := json.RawMessage(`{"version":1,"widgets":[{"id":"a","type":"stat.total","w":3,"h":1}]}`)
	var got *models.Dashboard
	db := &mockDB{
		getDashboardFn: func(ctx context.Context, id int64) (*models.Dashboard, error) {
			return &models.Dashboard{ID: id, UserID: testUserID, Name: "Old", Layout: layout, Shared: true}, nil
		},
		saveDashboardFn: func(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
			got = d
			return d, nil
		},
	}
	w, req := makeRequestWithParams("PUT", "/api/v1/dashboards/3", `{"name":"  Renamed  "}`, "id", "3")
	NewDashboardHandler(db).Save(w, dashboardAs(req, "viewer", models.PermDevicesRead))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got.Name != "Renamed" || string(got.Layout) != string(layout) || !got.Shared {
		t.Fatalf("omitted fields changed: %+v", got)
	}
}

func TestDashboard_SharingNeedsWriteAccess(t *testing.T) {
	db := &mockDB{saveDashboardFn: func(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) { return d, nil }}
	h := NewDashboardHandler(db)
	body := `{"name":"Team","shared":true}`

	w, req := makeRequestWithParams("POST", "/api/v1/dashboards", body)
	h.Save(w, dashboardAs(req, "viewer", models.PermDevicesRead))
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer sharing: expected 403, got %d", w.Code)
	}
	w, req = makeRequestWithParams("POST", "/api/v1/dashboards", `{"name":"Mine"}`)
	h.Save(w, dashboardAs(req, "viewer", models.PermDevicesRead))
	if w.Code != http.StatusOK {
		t.Fatalf("viewer private dashboard: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"widgets":[]`) {
		t.Fatalf("new dashboard should get an empty layout: %s", w.Body.String())
	}
	w, req = makeRequestWithParams("POST", "/api/v1/dashboards", body)
	h.Save(w, dashboardAs(req, "network_admin", models.PermDevicesWrite))
	if w.Code != http.StatusOK {
		t.Fatalf("operator sharing: expected 200, got %d", w.Code)
	}
}

func TestDashboard_Validation(t *testing.T) {
	db := &mockDB{saveDashboardFn: func(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) { return d, nil }}
	h := NewDashboardHandler(db)
	cases := map[string]string{
		"missing name":    `{"layout":{"version":1,"widgets":[]}}`,
		"long name":       `{"name":"` + strings.Repeat("x", 101) + `"}`,
		"legacy array":    `{"name":"x","layout":[]}`,
		"bad version":     `{"name":"x","layout":{"version":2,"widgets":[]}}`,
		"wide widget":     `{"name":"x","layout":{"version":1,"widgets":[{"id":"a","type":"stat.total","w":13,"h":1}]}}`,
		"duplicate id":    `{"name":"x","layout":{"version":1,"widgets":[{"id":"a","type":"t","w":1,"h":1},{"id":"a","type":"t","w":1,"h":1}]}}`,
		"bad type":        `{"name":"x","layout":{"version":1,"widgets":[{"id":"a","type":"<script>","w":1,"h":1}]}}`,
		"array config":    `{"name":"x","layout":{"version":1,"widgets":[{"id":"a","type":"t","w":1,"h":1,"config":[1]}]}}`,
		"oversized total": `{"name":"x","layout":{"version":1,"widgets":[],"pad":"` + strings.Repeat("x", 70<<10) + `"}}`,
	}
	// 49 widgets with distinct ids.
	var ws []string
	for i := 0; i < models.DashboardMaxWidgets+1; i++ {
		ws = append(ws, `{"id":"w`+strings.Repeat("x", i%3)+string(rune('a'+i%26))+string(rune('a'+i/26))+`","type":"t","w":1,"h":1}`)
	}
	cases["too many"] = `{"name":"x","layout":{"version":1,"widgets":[` + strings.Join(ws, ",") + `]}}`

	for name, body := range cases {
		w, req := makeRequestWithParams("POST", "/api/v1/dashboards", body)
		h.Save(w, dashboardAs(req, "admin", models.PermDevicesWrite))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d: %s", name, w.Code, w.Body.String())
		}
	}

	ok := `{"name":"Ops","layout":{"version":1,"extra":true,"widgets":[{"id":"a","type":"device.history","w":6,"h":2,"config":{"deviceIds":[1,2]}}]}}`
	w, req := makeRequestWithParams("POST", "/api/v1/dashboards", ok)
	h.Save(w, dashboardAs(req, "admin", models.PermDevicesWrite))
	if w.Code != http.StatusOK {
		t.Fatalf("valid layout: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "extra") {
		t.Fatalf("unknown layout fields must not be stored: %s", w.Body.String())
	}
}

func TestDashboard_Default(t *testing.T) {
	var set *int64
	called := false
	db := &mockDB{
		getDashboardFn: func(ctx context.Context, id int64) (*models.Dashboard, error) {
			return &models.Dashboard{ID: id, UserID: otherUserID, Shared: id == 1}, nil
		},
		setDefaultDashboardFn: func(ctx context.Context, userID int64, id *int64) error {
			called, set = true, id
			return nil
		},
	}
	h := NewDashboardHandler(db)
	w, req := makeRequestWithParams("PUT", "/api/v1/dashboards/1/default", "", "id", "1")
	h.SetDefault(w, dashboardAs(req, "viewer", models.PermDevicesRead))
	if w.Code != http.StatusOK || set == nil || *set != 1 {
		t.Fatalf("default to shared dashboard: %d %v", w.Code, set)
	}
	called = false
	w, req = makeRequestWithParams("PUT", "/api/v1/dashboards/2/default", "", "id", "2")
	h.SetDefault(w, dashboardAs(req, "viewer", models.PermDevicesRead))
	if w.Code != http.StatusNotFound || called {
		t.Fatalf("default to another user's private dashboard: expected 404, got %d", w.Code)
	}
	w, req = makeRequestWithParams("DELETE", "/api/v1/dashboards/default", "")
	h.ClearDefault(w, dashboardAs(req, "viewer", models.PermDevicesRead))
	if w.Code != http.StatusOK || set != nil {
		t.Fatalf("clear default: %d %v", w.Code, set)
	}
}
