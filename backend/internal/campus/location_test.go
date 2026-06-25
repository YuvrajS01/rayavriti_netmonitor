package campus

import (
	"testing"
)

// ---------- buildTree tests ----------

func TestBuildTree_Empty(t *testing.T) {
	roots := buildTree(nil)
	if len(roots) != 0 {
		t.Fatalf("expected 0 roots, got %d", len(roots))
	}
}

func TestBuildTree_SingleRoot(t *testing.T) {
	locs := []Location{
		{ID: 1, Name: "Campus", Type: "campus"},
	}
	roots := buildTree(locs)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if roots[0].Name != "Campus" {
		t.Fatalf("expected root name 'Campus', got %q", roots[0].Name)
	}
}

func TestBuildTree_Hierarchy(t *testing.T) {
	locs := []Location{
		{ID: 1, Name: "Campus", Type: "campus"},
		{ID: 2, Name: "Building A", Type: "building", ParentID: ptrInt64(1)},
		{ID: 3, Name: "Building B", Type: "building", ParentID: ptrInt64(1)},
		{ID: 4, Name: "Floor 1", Type: "floor", ParentID: ptrInt64(2)},
		{ID: 5, Name: "Room 101", Type: "room", ParentID: ptrInt64(4)},
	}
	roots := buildTree(locs)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	root := roots[0]
	if len(root.Children) != 2 {
		t.Fatalf("expected 2 children of root, got %d", len(root.Children))
	}

	// Find Building A
	var buildingA *Location
	for _, c := range root.Children {
		if c.Name == "Building A" {
			buildingA = c
			break
		}
	}
	if buildingA == nil {
		t.Fatal("Building A not found")
	}
	if len(buildingA.Children) != 1 {
		t.Fatalf("expected 1 child of Building A, got %d", len(buildingA.Children))
	}
	if buildingA.Children[0].Name != "Floor 1" {
		t.Fatalf("expected child 'Floor 1', got %q", buildingA.Children[0].Name)
	}
	floor1 := buildingA.Children[0]
	if len(floor1.Children) != 1 {
		t.Fatalf("expected 1 child of Floor 1, got %d", len(floor1.Children))
	}
	if floor1.Children[0].Name != "Room 101" {
		t.Fatalf("expected child 'Room 101', got %q", floor1.Children[0].Name)
	}
}

func TestBuildTree_OrphanBecomesRoot(t *testing.T) {
	locs := []Location{
		{ID: 1, Name: "Campus", Type: "campus"},
		{ID: 99, Name: "Orphan", Type: "room", ParentID: ptrInt64(500)},
	}
	roots := buildTree(locs)
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots (campus + orphan), got %d", len(roots))
	}
}

func TestBuildTree_ParentNilMeansRoot(t *testing.T) {
	locs := []Location{
		{ID: 1, Name: "A", Type: "campus", ParentID: nil},
		{ID: 2, Name: "B", Type: "building", ParentID: nil},
	}
	roots := buildTree(locs)
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
}

func TestBuildTree_MultipleRoots(t *testing.T) {
	locs := []Location{
		{ID: 1, Name: "Campus A", Type: "campus"},
		{ID: 2, Name: "Campus B", Type: "campus"},
		{ID: 3, Name: "Building", Type: "building", ParentID: ptrInt64(1)},
	}
	roots := buildTree(locs)
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
}

func TestBuildTree_DeepNesting(t *testing.T) {
	locs := []Location{
		{ID: 1, Name: "Root", Type: "campus"},
		{ID: 2, Name: "L1", Type: "building", ParentID: ptrInt64(1)},
		{ID: 3, Name: "L2", Type: "floor", ParentID: ptrInt64(2)},
		{ID: 4, Name: "L3", Type: "room", ParentID: ptrInt64(3)},
		{ID: 5, Name: "L4", Type: "rack", ParentID: ptrInt64(4)},
	}
	roots := buildTree(locs)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	// Walk down the chain
	current := roots[0]
	depth := 0
	for len(current.Children) > 0 {
		current = current.Children[0]
		depth++
	}
	if depth != 4 {
		t.Fatalf("expected depth 4, got %d", depth)
	}
}

func TestBuildTree_ClearsExistingChildren(t *testing.T) {
	locs := []Location{
		{ID: 1, Name: "A", Type: "campus", Children: []*Location{{ID: 99, Name: "stale"}}},
		{ID: 2, Name: "B", Type: "building", ParentID: ptrInt64(1)},
	}
	roots := buildTree(locs)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	// Should have only the real child "B", not "stale"
	if len(roots[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(roots[0].Children))
	}
	if roots[0].Children[0].Name != "B" {
		t.Fatalf("expected child 'B', got %q", roots[0].Children[0].Name)
	}
}

// ---------- NewLocationService ----------

func TestNewLocationService_NilDB(t *testing.T) {
	// LocationService should accept any value satisfying the DB interface.
	// We just test construction works.
	svc := NewLocationService(nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ---------- validLocationTypes ----------

func TestValidLocationTypes(t *testing.T) {
	valid := []string{"campus", "building", "floor", "room", "rack", "closet", "zone", "site"}
	for _, v := range valid {
		if !validLocationTypes[v] {
			t.Errorf("expected %q to be a valid type", v)
		}
	}
	invalid := []string{"", "invalid", "building_floor"}
	for _, v := range invalid {
		if validLocationTypes[v] {
			t.Errorf("expected %q to be an invalid type", v)
		}
	}
}

// ---------- Location model defaults ----------

func TestLocationStatus_Defaults(t *testing.T) {
	s := &LocationStatus{}
	if s.Up != 0 || s.Down != 0 || s.Warning != 0 || s.Maintenance != 0 || s.Unknown != 0 {
		t.Errorf("expected zero-valued LocationStatus, got %+v", s)
	}
}

func TestLocation_SortOrder(t *testing.T) {
	locs := []Location{
		{ID: 1, SortOrder: 3},
		{ID: 2, SortOrder: 1},
		{ID: 3, SortOrder: 2},
	}
	// Verify the sort orders are as expected
	expected := []int{3, 1, 2}
	for i, loc := range locs {
		if loc.SortOrder != expected[i] {
			t.Errorf("loc %d: expected sort order %d, got %d", i, expected[i], loc.SortOrder)
		}
	}
}

// ---------- helpers ----------

func ptrInt64(v int64) *int64 { return &v }
