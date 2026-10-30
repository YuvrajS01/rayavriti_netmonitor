package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/database"
)

// fakeStore is a minimal in-memory ResourceStore for the child-route tests.
type fakeStore struct {
	*mockDB
	rows      map[int64]map[string]any
	deleted   []int64
	lastQuery map[string]string
	created   map[string]any
}

func (f *fakeStore) ListResources(_ context.Context, _ string, filters map[string]string) ([]map[string]any, error) {
	f.lastQuery = filters
	return nil, nil
}
func (f *fakeStore) ListResourcesCursor(context.Context, string, map[string]string, string, int) ([]map[string]any, string, bool, error) {
	return nil, "", false, nil
}
func (f *fakeStore) GetResource(_ context.Context, _ string, id int64) (map[string]any, error) {
	return f.rows[id], nil
}
func (f *fakeStore) CreateResource(_ context.Context, _ string, values map[string]any) (map[string]any, error) {
	f.created = values
	return values, nil
}
func (f *fakeStore) UpdateResource(context.Context, string, int64, map[string]any) (map[string]any, error) {
	return nil, nil
}
func (f *fakeStore) DeleteResource(_ context.Context, _ string, id int64) error {
	f.deleted = append(f.deleted, id)
	return nil
}
func (f *fakeStore) ResourceSummary(context.Context) (database.ResourceSummary, error) {
	return database.ResourceSummary{}, nil
}

func newFakeStoreHandler() (*ResourceHandler, *fakeStore) {
	fs := &fakeStore{mockDB: &mockDB{}, rows: map[int64]map[string]any{
		5: {"id": int64(5), "device_id": int64(1)}, // contact link on device 1
	}}
	return &ResourceHandler{db: fs.mockDB, store: fs}, fs
}

func TestDeleteChild_OnlyDeletesRowsOfThatParent(t *testing.T) {
	h, fs := newFakeStoreHandler()
	del := h.DeleteChild("device_contacts", "cid", "device_id")

	w := httptest.NewRecorder()
	del(w, scopedRequest("DELETE", "/api/v1/devices/2/contacts/5", "", nil, "id", "2", "cid", "5"))
	if w.Code != http.StatusNotFound || len(fs.deleted) != 0 {
		t.Fatalf("deleted another device's contact link: status %d, deleted %v", w.Code, fs.deleted)
	}

	w = httptest.NewRecorder()
	del(w, scopedRequest("DELETE", "/api/v1/devices/1/contacts/5", "", nil, "id", "1", "cid", "5"))
	if w.Code != http.StatusOK || len(fs.deleted) != 1 || fs.deleted[0] != 5 {
		t.Fatalf("expected row 5 deleted, got status %d, deleted %v", w.Code, fs.deleted)
	}
}

func TestListByParent_FiltersOnURLParent(t *testing.T) {
	h, fs := newFakeStoreHandler()
	w := httptest.NewRecorder()
	// A caller-supplied filter for the parent column must not win.
	h.ListByParent("escalation_steps", "policy_id")(w,
		scopedRequest("GET", "/api/v1/escalation-policies/3/steps?policyId=99", "", nil, "id", "3"))
	if fs.lastQuery["policy_id"] != "3" {
		t.Fatalf("filters = %v, want policy_id=3", fs.lastQuery)
	}
}

func TestCreateUnderParent_ForcesParentFromURL(t *testing.T) {
	h, fs := newFakeStoreHandler()
	w := httptest.NewRecorder()
	h.CreateUnderParent("device_contacts", "device_id")(w,
		scopedRequest("POST", "/api/v1/devices/4/contacts", `{"deviceId":99,"contact_id":7}`, nil, "id", "4"))
	if got := fs.created["device_id"]; got != float64(4) && got != int64(4) {
		t.Fatalf("device_id = %v (%T), want 4 from the URL", got, got)
	}
}
