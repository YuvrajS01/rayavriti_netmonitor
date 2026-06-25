// Package campus provides services for managing the physical campus topology,
// including location hierarchy and device dependency trees.
package campus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB abstracts the pgx query interface so the campus package can run SQL
// without depending on the full database.Database interface.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// validLocationTypes enumerates the allowed values for Location.Type.
var validLocationTypes = map[string]bool{
	"campus":   true,
	"building": true,
	"floor":    true,
	"room":     true,
	"rack":     true,
	"closet":   true,
	"zone":     true,
	"site":     true,
}

// ---------- Models ----------------------------------------------------------

// Location represents a node in the physical location hierarchy.
type Location struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	ParentID        *int64          `json:"parent_id"`
	Code            string          `json:"code"`
	Description     string          `json:"description"`
	Address         string          `json:"address"`
	Latitude        *float64        `json:"latitude"`
	Longitude       *float64        `json:"longitude"`
	FloorNumber     *int            `json:"floor_number"`
	ContactPersonID *int64          `json:"contact_person_id"`
	Metadata        json.RawMessage `json:"metadata"`
	SortOrder       int             `json:"sort_order"`
	Enabled         bool            `json:"enabled"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	Children        []*Location     `json:"children,omitempty"`
	DeviceCount     int             `json:"device_count"`
	Status          *LocationStatus `json:"status,omitempty"`
}

// LocationStatus holds aggregated device-status counts for a location
// and all of its descendants.
type LocationStatus struct {
	Up          int `json:"up"`
	Down        int `json:"down"`
	Warning     int `json:"warning"`
	Maintenance int `json:"maintenance"`
	Unknown     int `json:"unknown"`
}

// ---------- Service ---------------------------------------------------------

// LocationService provides typed operations for the location hierarchy.
type LocationService struct {
	db DB
}

// NewLocationService returns a LocationService backed by db.
func NewLocationService(db DB) *LocationService {
	return &LocationService{db: db}
}

// ---------- Read operations -------------------------------------------------

// locationColumns is the SELECT column list shared by all location queries.
const locationColumns = `id, name, type, parent_id, code, description, address,
	latitude, longitude, floor_number, contact_person_id, metadata,
	sort_order, enabled, created_at, updated_at`

// scanLocation scans a single row into a Location.
func scanLocation(row pgx.Row) (*Location, error) {
	var l Location
	var meta []byte
	var code, desc, addr *string
	err := row.Scan(
		&l.ID, &l.Name, &l.Type, &l.ParentID,
		&code, &desc, &addr,
		&l.Latitude, &l.Longitude, &l.FloorNumber,
		&l.ContactPersonID, &meta,
		&l.SortOrder, &l.Enabled, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if code != nil {
		l.Code = *code
	}
	if desc != nil {
		l.Description = *desc
	}
	if addr != nil {
		l.Address = *addr
	}
	if meta != nil {
		l.Metadata = meta
	} else {
		l.Metadata = json.RawMessage(`{}`)
	}
	return &l, nil
}

// scanLocations collects all rows from a query into a Location slice.
func scanLocations(rows pgx.Rows) ([]Location, error) {
	defer rows.Close()
	var out []Location
	for rows.Next() {
		var l Location
		var meta []byte
		var code, desc, addr *string
		if err := rows.Scan(
			&l.ID, &l.Name, &l.Type, &l.ParentID,
			&code, &desc, &addr,
			&l.Latitude, &l.Longitude, &l.FloorNumber,
			&l.ContactPersonID, &meta,
			&l.SortOrder, &l.Enabled, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan location row: %w", err)
		}
		if code != nil {
			l.Code = *code
		}
		if desc != nil {
			l.Description = *desc
		}
		if addr != nil {
			l.Address = *addr
		}
		if meta != nil {
			l.Metadata = meta
		} else {
			l.Metadata = json.RawMessage(`{}`)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetAll returns a flat list of every location ordered by sort_order.
func (s *LocationService) GetAll(ctx context.Context) ([]Location, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+locationColumns+` FROM locations ORDER BY sort_order, name`)
	if err != nil {
		return nil, fmt.Errorf("query locations: %w", err)
	}
	return scanLocations(rows)
}

// GetByID returns a single location by its primary key.
func (s *LocationService) GetByID(ctx context.Context, id int64) (*Location, error) {
	row := s.db.QueryRow(ctx,
		`SELECT `+locationColumns+` FROM locations WHERE id = $1`, id)
	loc, err := scanLocation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("location %d not found", id)
	}
	return loc, err
}

// ---------- Tree operations -------------------------------------------------

// buildTree constructs a tree from a flat slice and returns root nodes.
func buildTree(locs []Location) []*Location {
	index := make(map[int64]*Location, len(locs))
	for i := range locs {
		locs[i].Children = nil // ensure clean slate
		index[locs[i].ID] = &locs[i]
	}

	var roots []*Location
	for i := range locs {
		loc := &locs[i]
		if loc.ParentID != nil {
			if parent, ok := index[*loc.ParentID]; ok {
				parent.Children = append(parent.Children, loc)
				continue
			}
		}
		roots = append(roots, loc)
	}
	return roots
}

// GetTree returns the full location hierarchy as a tree of root nodes.
func (s *LocationService) GetTree(ctx context.Context) ([]*Location, error) {
	locs, err := s.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	return buildTree(locs), nil
}

// GetSubtree loads a location and all of its descendants as a tree.
func (s *LocationService) GetSubtree(ctx context.Context, id int64) (*Location, error) {
	// Use a recursive CTE to grab the subtree in one query.
	rows, err := s.db.Query(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT `+locationColumns+` FROM locations WHERE id = $1
			UNION ALL
			SELECT l.`+locationColumns+`
			  FROM locations l
			  JOIN subtree s ON l.parent_id = s.id
		)
		SELECT `+locationColumns+` FROM subtree ORDER BY sort_order, name`, id)
	if err != nil {
		return nil, fmt.Errorf("query subtree: %w", err)
	}

	locs, err := scanLocations(rows)
	if err != nil {
		return nil, err
	}
	if len(locs) == 0 {
		return nil, fmt.Errorf("location %d not found", id)
	}

	roots := buildTree(locs)
	// The first root should be the requested location.
	for _, r := range roots {
		if r.ID == id {
			return r, nil
		}
	}
	return roots[0], nil
}

// ---------- Write operations ------------------------------------------------

// Create inserts a new location after validating its type and code uniqueness.
func (s *LocationService) Create(ctx context.Context, loc *Location) (*Location, error) {
	if !validLocationTypes[loc.Type] {
		return nil, fmt.Errorf("invalid location type %q", loc.Type)
	}

	if loc.Code != "" {
		var exists bool
		err := s.db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM locations WHERE code = $1)`, loc.Code).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("check code uniqueness: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("location code %q already exists", loc.Code)
		}
	}

	if loc.Metadata == nil {
		loc.Metadata = json.RawMessage(`{}`)
	}

	row := s.db.QueryRow(ctx, `
		INSERT INTO locations (name, type, parent_id, code, description, address,
			latitude, longitude, floor_number, contact_person_id, metadata,
			sort_order, enabled, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13, now(), now())
		RETURNING `+locationColumns,
		loc.Name, loc.Type, loc.ParentID, loc.Code, loc.Description, loc.Address,
		loc.Latitude, loc.Longitude, loc.FloorNumber, loc.ContactPersonID, loc.Metadata,
		loc.SortOrder, loc.Enabled,
	)

	created, err := scanLocation(row)
	if err != nil {
		return nil, fmt.Errorf("insert location: %w", err)
	}
	slog.Info("location created", "id", created.ID, "name", created.Name)
	return created, nil
}

// Update modifies an existing location, setting updated_at to now().
func (s *LocationService) Update(ctx context.Context, id int64, loc *Location) (*Location, error) {
	if loc.Type != "" && !validLocationTypes[loc.Type] {
		return nil, fmt.Errorf("invalid location type %q", loc.Type)
	}

	if loc.Metadata == nil {
		loc.Metadata = json.RawMessage(`{}`)
	}

	if loc.Code != "" {
		var conflictID int64
		err := s.db.QueryRow(ctx,
			`SELECT id FROM locations WHERE code = $1 AND id != $2`, loc.Code, id).Scan(&conflictID)
		if err == nil {
			return nil, fmt.Errorf("location code %q already exists", loc.Code)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check code uniqueness: %w", err)
		}
	}

	row := s.db.QueryRow(ctx, `
		UPDATE locations SET
			name = $2, type = $3, parent_id = $4, code = $5,
			description = $6, address = $7, latitude = $8, longitude = $9,
			floor_number = $10, contact_person_id = $11, metadata = $12,
			sort_order = $13, enabled = $14, updated_at = now()
		WHERE id = $1
		RETURNING `+locationColumns,
		id,
		loc.Name, loc.Type, loc.ParentID, loc.Code,
		loc.Description, loc.Address, loc.Latitude, loc.Longitude,
		loc.FloorNumber, loc.ContactPersonID, loc.Metadata,
		loc.SortOrder, loc.Enabled,
	)

	updated, err := scanLocation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("location %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("update location: %w", err)
	}
	slog.Info("location updated", "id", id)
	return updated, nil
}

// Delete removes a location. Children of the deleted location are re-parented
// to its parent (or set to NULL if it was a root).
func (s *LocationService) Delete(ctx context.Context, id int64) error {
	// Re-parent children first.
	loc, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(ctx,
		`UPDATE locations SET parent_id = $1 WHERE parent_id = $2`,
		loc.ParentID, id)
	if err != nil {
		return fmt.Errorf("re-parent children: %w", err)
	}

	tag, err := s.db.Exec(ctx, `DELETE FROM locations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete location: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("location %d not found", id)
	}
	slog.Info("location deleted", "id", id)
	return nil
}

// ---------- Move with circular-dependency check -----------------------------

// Move re-parents a location, first ensuring the move would not create a cycle.
func (s *LocationService) Move(ctx context.Context, id int64, newParentID *int64) error {
	if newParentID == nil {
		// Moving to root is always safe.
		_, err := s.db.Exec(ctx,
			`UPDATE locations SET parent_id = NULL, updated_at = now() WHERE id = $1`, id)
		if err != nil {
			return fmt.Errorf("move to root: %w", err)
		}
		slog.Info("location moved to root", "id", id)
		return nil
	}

	if *newParentID == id {
		return errors.New("a location cannot be its own parent")
	}

	// Walk from newParentID up to root; if we encounter id, it's a cycle.
	current := *newParentID
	for {
		var parentID *int64
		err := s.db.QueryRow(ctx,
			`SELECT parent_id FROM locations WHERE id = $1`, current).Scan(&parentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("parent location %d not found", current)
		}
		if err != nil {
			return fmt.Errorf("cycle check: %w", err)
		}
		if parentID == nil {
			break // reached root, no cycle
		}
		if *parentID == id {
			return fmt.Errorf("moving location %d under %d would create a circular dependency", id, *newParentID)
		}
		current = *parentID
	}

	_, err := s.db.Exec(ctx,
		`UPDATE locations SET parent_id = $1, updated_at = now() WHERE id = $2`,
		newParentID, id)
	if err != nil {
		return fmt.Errorf("move location: %w", err)
	}
	slog.Info("location moved", "id", id, "newParentId", *newParentID)
	return nil
}

// ---------- Device queries --------------------------------------------------

// GetDevicesAtLocation returns device IDs at the given location. When
// recursive is true the result also includes devices at all descendant
// locations.
func (s *LocationService) GetDevicesAtLocation(ctx context.Context, locationID int64, recursive bool) ([]int64, error) {
	var query string
	if recursive {
		query = `
			WITH RECURSIVE subtree AS (
				SELECT id FROM locations WHERE id = $1
				UNION ALL
				SELECT l.id FROM locations l JOIN subtree s ON l.parent_id = s.id
			)
			SELECT d.id FROM devices d WHERE d.location_id IN (SELECT id FROM subtree)
			ORDER BY d.id`
	} else {
		query = `SELECT id FROM devices WHERE location_id = $1 ORDER BY id`
	}

	rows, err := s.db.Query(ctx, query, locationID)
	if err != nil {
		return nil, fmt.Errorf("get devices at location: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan device id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ---------- Status aggregation ----------------------------------------------

// GetLocationStatus returns aggregated device-status counts for a location
// and all of its descendants.
func (s *LocationService) GetLocationStatus(ctx context.Context, locationID int64) (*LocationStatus, error) {
	row := s.db.QueryRow(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT id FROM locations WHERE id = $1
			UNION ALL
			SELECT l.id FROM locations l JOIN subtree s ON l.parent_id = s.id
		)
		SELECT
			COALESCE(SUM(CASE WHEN d.status = 'up'          THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN d.status = 'down'        THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN d.status = 'warning'     THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN d.status = 'maintenance' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN d.status NOT IN ('up','down','warning','maintenance')
				THEN 1 ELSE 0 END), 0)
		FROM devices d
		WHERE d.location_id IN (SELECT id FROM subtree)`,
		locationID)

	var st LocationStatus
	if err := row.Scan(&st.Up, &st.Down, &st.Warning, &st.Maintenance, &st.Unknown); err != nil {
		return nil, fmt.Errorf("get location status: %w", err)
	}
	return &st, nil
}

// ---------- Tree with status ------------------------------------------------

// deviceRow is an internal struct for the status-enriched tree query.
type deviceRow struct {
	LocationID int64
	Status     string
}

// GetTreeWithStatus builds the full location tree with device counts and
// status aggregated from each node's subtree.
func (s *LocationService) GetTreeWithStatus(ctx context.Context) ([]*Location, error) {
	locs, err := s.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch all devices with their location and status.
	rows, err := s.db.Query(ctx,
		`SELECT location_id, status FROM devices WHERE location_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("query device statuses: %w", err)
	}
	defer rows.Close()

	// Aggregate per-location status.
	statusMap := make(map[int64]*LocationStatus)
	countMap := make(map[int64]int)
	for rows.Next() {
		var d deviceRow
		if err := rows.Scan(&d.LocationID, &d.Status); err != nil {
			return nil, fmt.Errorf("scan device row: %w", err)
		}
		st, ok := statusMap[d.LocationID]
		if !ok {
			st = &LocationStatus{}
			statusMap[d.LocationID] = st
		}
		countMap[d.LocationID]++
		switch d.Status {
		case "up":
			st.Up++
		case "down":
			st.Down++
		case "warning":
			st.Warning++
		case "maintenance":
			st.Maintenance++
		default:
			st.Unknown++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tree := buildTree(locs)

	// Propagate counts upward: post-order traversal.
	var propagate func(n *Location)
	propagate = func(n *Location) {
		if st, ok := statusMap[n.ID]; ok {
			n.Status = st
		} else {
			n.Status = &LocationStatus{}
		}
		n.DeviceCount = countMap[n.ID]

		for _, child := range n.Children {
			propagate(child)
			n.DeviceCount += child.DeviceCount
			n.Status.Up += child.Status.Up
			n.Status.Down += child.Status.Down
			n.Status.Warning += child.Status.Warning
			n.Status.Maintenance += child.Status.Maintenance
			n.Status.Unknown += child.Status.Unknown
		}
	}
	for _, root := range tree {
		propagate(root)
	}

	return tree, nil
}
