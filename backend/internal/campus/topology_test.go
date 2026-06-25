package campus

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRow struct {
	values []any
	err    error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, v := range r.values {
		if i < len(dest) {
			setDest(dest[i], v)
		}
	}
	return nil
}

type mockRows struct {
	nodes   []mockRow
	current int
}

func (r *mockRows) Close() {}
func (r *mockRows) Next() bool {
	return r.current < len(r.nodes)
}
func (r *mockRows) Scan(dest ...any) error {
	if r.current >= len(r.nodes) {
		return fmt.Errorf("no more rows")
	}
	node := r.nodes[r.current]
	r.current++
	for i, v := range node.values {
		if i < len(dest) {
			setDest(dest[i], v)
		}
	}
	return node.err
}
func (r *mockRows) Err() error                                   { return nil }
func (r *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) Values() ([]any, error)                       { return nil, nil }
func (r *mockRows) RawValues() [][]byte                          { return nil }
func (r *mockRows) Conn() *pgx.Conn                              { return nil }

type mockDB struct {
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *mockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{}
}

func (m *mockDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

type mockRow2 struct {
	vals []any
	err  error
}

func (r *mockRow2) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, v := range r.vals {
		if i < len(dest) {
			setDest(dest[i], v)
		}
	}
	return nil
}

func setDest(dest any, val any) {
	switch d := dest.(type) {
	case *int64:
		if v, ok := val.(int64); ok {
			*d = v
		}
	case *string:
		if v, ok := val.(string); ok {
			*d = v
		}
	case *bool:
		if v, ok := val.(bool); ok {
			*d = v
		}
	case **int64:
		if val == nil {
			*d = nil
		} else {
			switch v := val.(type) {
			case int64:
				*d = &v
			case *int64:
				*d = v
			}
		}
	case **string:
		if val == nil {
			*d = nil
		} else {
			switch v := val.(type) {
			case string:
				*d = &v
			case *string:
				*d = v
			}
		}
	}
}

// ── BuildDependencyTree ─────────────────────────────────────────────────────

func TestBuildDependencyTree_SingleRoot(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Router", "10.0.0.1", "up", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	roots, err := svc.BuildDependencyTree(context.Background())
	require.NoError(t, err)
	assert.Len(t, roots, 1)
	assert.Equal(t, int64(1), roots[0].DeviceID)
	assert.Equal(t, "Router", roots[0].Name)
}

func TestBuildDependencyTree_ParentChild(t *testing.T) {
	t.Parallel()
	parentID := int64(1)
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			cat1 := "core"
			cat2 := "access"
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Router", "10.0.0.1", "up", &cat1, (*int64)(nil), (*int64)(nil), (*string)(nil)}},
					{values: []any{int64(2), "Switch", "10.0.0.2", "up", &cat2, (*int64)(nil), &parentID, (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	roots, err := svc.BuildDependencyTree(context.Background())
	require.NoError(t, err)
	assert.Len(t, roots, 1)
	assert.Equal(t, int64(1), roots[0].DeviceID)
	require.Len(t, roots[0].Children, 1)
	assert.Equal(t, int64(2), roots[0].Children[0].DeviceID)
}

func TestBuildDependencyTree_MultipleRoots(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Router-A", "10.0.0.1", "up", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
					{values: []any{int64(2), "Router-B", "10.0.0.2", "up", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	roots, err := svc.BuildDependencyTree(context.Background())
	require.NoError(t, err)
	assert.Len(t, roots, 2)
}

func TestBuildDependencyTree_OrphanChild(t *testing.T) {
	t.Parallel()
	orphanParent := int64(999)
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Switch", "10.0.0.1", "up", (*string)(nil), (*int64)(nil), &orphanParent, (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	roots, err := svc.BuildDependencyTree(context.Background())
	require.NoError(t, err)
	assert.Len(t, roots, 1)
	assert.Equal(t, int64(1), roots[0].DeviceID)
}

func TestBuildDependencyTree_Empty(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{nodes: []mockRow{}}, nil
		},
	}
	svc := NewTopologyService(db)
	roots, err := svc.BuildDependencyTree(context.Background())
	require.NoError(t, err)
	assert.Empty(t, roots)
}

func TestBuildDependencyTree_QueryError(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	svc := NewTopologyService(db)
	_, err := svc.BuildDependencyTree(context.Background())
	require.Error(t, err)
}

func TestBuildDependencyTree_DeepNesting(t *testing.T) {
	t.Parallel()
	p1 := int64(1)
	p2 := int64(2)
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Root", "10.0.0.1", "up", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
					{values: []any{int64(2), "Level1", "10.0.0.2", "up", (*string)(nil), (*int64)(nil), &p1, (*string)(nil)}},
					{values: []any{int64(3), "Level2", "10.0.0.3", "up", (*string)(nil), (*int64)(nil), &p2, (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	roots, err := svc.BuildDependencyTree(context.Background())
	require.NoError(t, err)
	assert.Len(t, roots, 1)
	require.Len(t, roots[0].Children, 1)
	require.Len(t, roots[0].Children[0].Children, 1)
	assert.Equal(t, "Level2", roots[0].Children[0].Children[0].Name)
}

func TestBuildDependencyTree_DependencyPort(t *testing.T) {
	t.Parallel()
	depPort := "8080"
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Web", "10.0.0.1", "up", (*string)(nil), (*int64)(nil), (*int64)(nil), &depPort}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	roots, err := svc.BuildDependencyTree(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "8080", roots[0].DependencyPort)
}

// ── CheckSuppression ────────────────────────────────────────────────────────

func TestCheckSuppression_ParentDown(t *testing.T) {
	t.Parallel()
	var parentID *int64
	db := &mockDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			id := args[0].(int64)
			switch id {
			case 2:
				p := int64(1)
				return &mockRow2{vals: []any{"Switch", "10.0.0.2", "up", &p}}
			case 1:
				return &mockRow2{vals: []any{"Router", "10.0.0.1", "down", parentID}}
			}
			return &mockRow2{err: fmt.Errorf("not found")}
		},
	}
	svc := NewTopologyService(db)
	result, err := svc.CheckSuppression(context.Background(), 2)
	require.NoError(t, err)
	assert.True(t, result.ShouldSuppress)
	assert.Equal(t, "parent_down", result.Reason)
	assert.NotNil(t, result.RootCauseDevice)
	assert.Equal(t, int64(1), result.RootCauseDevice.DeviceID)
}

func TestCheckSuppression_AllUp(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			id := args[0].(int64)
			switch id {
			case 2:
				p := int64(1)
				return &mockRow2{vals: []any{"Switch", "10.0.0.2", "up", &p}}
			case 1:
				return &mockRow2{vals: []any{"Router", "10.0.0.1", "up", (*int64)(nil)}}
			}
			return &mockRow2{err: fmt.Errorf("not found")}
		},
	}
	svc := NewTopologyService(db)
	result, err := svc.CheckSuppression(context.Background(), 2)
	require.NoError(t, err)
	assert.False(t, result.ShouldSuppress)
}

func TestCheckSuppression_RootDevice(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow2{vals: []any{"Router", "10.0.0.1", "down", (*int64)(nil)}}
		},
	}
	svc := NewTopologyService(db)
	result, err := svc.CheckSuppression(context.Background(), 1)
	require.NoError(t, err)
	assert.False(t, result.ShouldSuppress)
}

func TestCheckSuppression_ThreeLevelsDeep_ParentDown(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			id := args[0].(int64)
			switch id {
			case 3:
				p := int64(2)
				return &mockRow2{vals: []any{"AP", "10.0.0.3", "up", &p}}
			case 2:
				p := int64(1)
				return &mockRow2{vals: []any{"Switch", "10.0.0.2", "up", &p}}
			case 1:
				return &mockRow2{vals: []any{"Router", "10.0.0.1", "down", (*int64)(nil)}}
			}
			return &mockRow2{err: fmt.Errorf("not found")}
		},
	}
	svc := NewTopologyService(db)
	result, err := svc.CheckSuppression(context.Background(), 3)
	require.NoError(t, err)
	assert.True(t, result.ShouldSuppress)
	assert.Equal(t, "parent_down", result.Reason)
	assert.Equal(t, int64(1), result.RootCauseDevice.DeviceID)
}

func TestCheckSuppression_DeviceNotFound(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow2{err: pgx.ErrNoRows}
		},
	}
	svc := NewTopologyService(db)
	result, err := svc.CheckSuppression(context.Background(), 999)
	require.NoError(t, err)
	assert.False(t, result.ShouldSuppress)
}

func TestCheckSuppression_CircularReference(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			id := args[0].(int64)
			switch id {
			case 1:
				p := int64(2)
				return &mockRow2{vals: []any{"A", "10.0.0.1", "up", &p}}
			case 2:
				p := int64(1)
				return &mockRow2{vals: []any{"B", "10.0.0.2", "up", &p}}
			}
			return &mockRow2{err: fmt.Errorf("not found")}
		},
	}
	svc := NewTopologyService(db)
	result, err := svc.CheckSuppression(context.Background(), 1)
	require.NoError(t, err)
	assert.False(t, result.ShouldSuppress)
}

// ── GetRootCauseOutages ─────────────────────────────────────────────────────

func TestGetRootCauseOutages_SingleRootCause(t *testing.T) {
	t.Parallel()
	p1 := int64(1)
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Router", "10.0.0.1", "down", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
					{values: []any{int64(2), "Switch", "10.0.0.2", "down", (*string)(nil), (*int64)(nil), &p1, (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	outages, err := svc.GetRootCauseOutages(context.Background())
	require.NoError(t, err)
	require.Len(t, outages, 1)
	assert.Equal(t, int64(1), outages[0].Device.DeviceID)
	assert.Equal(t, 1, outages[0].AffectedCount)
}

func TestGetRootCauseOutages_NoDownDevices(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Router", "10.0.0.1", "up", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	outages, err := svc.GetRootCauseOutages(context.Background())
	require.NoError(t, err)
	assert.Empty(t, outages)
}

func TestGetRootCauseOutages_MultipleRootCauses(t *testing.T) {
	t.Parallel()
	p1 := int64(1)
	p3 := int64(3)
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Router-A", "10.0.0.1", "down", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
					{values: []any{int64(2), "Switch-A", "10.0.0.2", "down", (*string)(nil), (*int64)(nil), &p1, (*string)(nil)}},
					{values: []any{int64(3), "Router-B", "10.0.0.3", "down", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
					{values: []any{int64(4), "Switch-B", "10.0.0.4", "down", (*string)(nil), (*int64)(nil), &p3, (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	outages, err := svc.GetRootCauseOutages(context.Background())
	require.NoError(t, err)
	assert.Len(t, outages, 2)
}

func TestGetRootCauseOutages_ByCategory(t *testing.T) {
	t.Parallel()
	p1 := int64(1)
	cat1 := "access"
	cat2 := "access"
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Router", "10.0.0.1", "down", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
					{values: []any{int64(2), "Switch-A", "10.0.0.2", "up", &cat1, (*int64)(nil), &p1, (*string)(nil)}},
					{values: []any{int64(3), "Switch-B", "10.0.0.3", "up", &cat2, (*int64)(nil), &p1, (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	outages, err := svc.GetRootCauseOutages(context.Background())
	require.NoError(t, err)
	require.Len(t, outages, 1)
	assert.Equal(t, 2, outages[0].AffectedCount)
	assert.Equal(t, 2, outages[0].AffectedByCategory["access"])
}

func TestGetRootCauseOutages_DownChildNotRootCause(t *testing.T) {
	t.Parallel()
	p1 := int64(1)
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Router", "10.0.0.1", "down", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
					{values: []any{int64(2), "Switch", "10.0.0.2", "down", (*string)(nil), (*int64)(nil), &p1, (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	outages, err := svc.GetRootCauseOutages(context.Background())
	require.NoError(t, err)
	require.Len(t, outages, 1)
	assert.Equal(t, int64(1), outages[0].Device.DeviceID)
	assert.NotEqual(t, int64(2), outages[0].Device.DeviceID)
}

// ── GetDeviceDependencies ───────────────────────────────────────────────────

func TestGetDeviceDependencies_AncestorsAndDescendants(t *testing.T) {
	t.Parallel()
	p1 := int64(1)
	p2 := int64(2)
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Router", "10.0.0.1", "up", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
					{values: []any{int64(2), "Switch", "10.0.0.2", "up", (*string)(nil), (*int64)(nil), &p1, (*string)(nil)}},
					{values: []any{int64(3), "AP", "10.0.0.3", "up", (*string)(nil), (*int64)(nil), &p2, (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	ancestors, descendants, err := svc.GetDeviceDependencies(context.Background(), 2)
	require.NoError(t, err)
	assert.Len(t, ancestors, 1)
	assert.Equal(t, int64(1), ancestors[0].DeviceID)
	assert.Len(t, descendants, 1)
	assert.Equal(t, int64(3), descendants[0].DeviceID)
}

func TestGetDeviceDependencies_RootDevice(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{
				nodes: []mockRow{
					{values: []any{int64(1), "Router", "10.0.0.1", "up", (*string)(nil), (*int64)(nil), (*int64)(nil), (*string)(nil)}},
				},
			}, nil
		},
	}
	svc := NewTopologyService(db)
	ancestors, descendants, err := svc.GetDeviceDependencies(context.Background(), 1)
	require.NoError(t, err)
	assert.Empty(t, ancestors)
	assert.Empty(t, descendants)
}

func TestGetDeviceDependencies_DeviceNotFound(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockRows{nodes: []mockRow{}}, nil
		},
	}
	svc := NewTopologyService(db)
	_, _, err := svc.GetDeviceDependencies(context.Background(), 999)
	require.Error(t, err)
}
