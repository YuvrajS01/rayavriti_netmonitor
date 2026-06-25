// Package importer provides CSV-based bulk device import for NetMonitor.
// It supports parsing, field-level validation with dry-run previews, and
// transactional execution that creates devices and their default sensors.
package importer

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB is the minimal database interface required by ImportService.
// It is satisfied by *pgxpool.Pool as well as pgx.Tx.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

// csvHeaders defines the expected column order in the import CSV.
var csvHeaders = []string{
	"name", "host", "protocol", "port", "device_category",
	"location_code", "parent_device_host", "mac_address",
	"asset_tag", "contact_email", "notes",
}

// macRegex validates the MAC address format XX:XX:XX:XX:XX:XX (case-insensitive hex).
var macRegex = regexp.MustCompile(`(?i)^([0-9A-F]{2}:){5}[0-9A-F]{2}$`)

// ---------- Types ----------

// ImportRow represents a single parsed row from the CSV file.
type ImportRow struct {
	LineNumber       int    `json:"lineNumber"`
	Name             string `json:"name"`
	Host             string `json:"host"`
	Protocol         string `json:"protocol"`
	Port             int    `json:"port"`
	DeviceCategory   string `json:"deviceCategory"`
	LocationCode     string `json:"locationCode"`
	ParentDeviceHost string `json:"parentDeviceHost"`
	MACAddress       string `json:"macAddress"`
	AssetTag         string `json:"assetTag"`
	ContactEmail     string `json:"contactEmail"`
	Notes            string `json:"notes"`

	// Resolved fields (filled during validation)
	ResolvedLocationID *int64 `json:"resolvedLocationId,omitempty"`
	ResolvedParentID   *int64 `json:"resolvedParentId,omitempty"`
}

// RowValidation contains the validation outcome for a single import row.
type RowValidation struct {
	Row         ImportRow `json:"row"`
	Valid       bool      `json:"valid"`
	Errors      []string  `json:"errors,omitempty"`
	Warnings    []string  `json:"warnings,omitempty"`
	IsDuplicate bool      `json:"isDuplicate"`
}

// ImportPreview summarises validation results across all rows, providing
// a dry-run overview before the caller commits the import.
type ImportPreview struct {
	TotalRows  int             `json:"totalRows"`
	Valid      int             `json:"valid"`
	Warnings   int             `json:"warnings"`
	Errors     int             `json:"errors"`
	Duplicates int             `json:"duplicates"`
	Rows       []RowValidation `json:"rows"`
}

// ImportResult contains the outcome of an executed import.
type ImportResult struct {
	DevicesCreated int      `json:"devicesCreated"`
	SensorsCreated int      `json:"sensorsCreated"`
	Errors         []string `json:"errors,omitempty"`
}

// ImportService orchestrates CSV parsing, validation, and import execution.
type ImportService struct {
	db DB
}

// NewImportService creates an ImportService backed by the given database connection.
func NewImportService(db DB) *ImportService {
	return &ImportService{db: db}
}

// ---------- ParseCSV ----------

// ParseCSV reads CSV data from reader, skips the header row, trims whitespace,
// and returns the parsed rows. Port values are converted to int; an unparseable
// port is treated as 0.
func (s *ImportService) ParseCSV(reader io.Reader) ([]ImportRow, error) {
	cr := csv.NewReader(reader)
	cr.TrimLeadingSpace = true
	cr.LazyQuotes = true

	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv parse error: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("csv file is empty")
	}

	// Validate header row.
	header := records[0]
	if len(header) < len(csvHeaders) {
		return nil, fmt.Errorf("csv header has %d columns, expected at least %d", len(header), len(csvHeaders))
	}
	for i, expected := range csvHeaders {
		got := strings.TrimSpace(strings.ToLower(header[i]))
		if got != expected {
			return nil, fmt.Errorf("csv header column %d: expected %q, got %q", i+1, expected, got)
		}
	}

	rows := make([]ImportRow, 0, len(records)-1)
	for i, rec := range records[1:] {
		if len(rec) < len(csvHeaders) {
			// Pad short rows with empty strings so field access is safe.
			padded := make([]string, len(csvHeaders))
			copy(padded, rec)
			rec = padded
		}

		port, _ := strconv.Atoi(strings.TrimSpace(rec[3]))

		rows = append(rows, ImportRow{
			LineNumber:       i + 2, // 1-indexed, +1 for header
			Name:             strings.TrimSpace(rec[0]),
			Host:             strings.TrimSpace(rec[1]),
			Protocol:         strings.TrimSpace(rec[2]),
			Port:             port,
			DeviceCategory:   strings.TrimSpace(rec[4]),
			LocationCode:     strings.TrimSpace(rec[5]),
			ParentDeviceHost: strings.TrimSpace(rec[6]),
			MACAddress:       strings.TrimSpace(rec[7]),
			AssetTag:         strings.TrimSpace(rec[8]),
			ContactEmail:     strings.TrimSpace(rec[9]),
			Notes:            strings.TrimSpace(rec[10]),
		})
	}

	return rows, nil
}

// ---------- Validate ----------

// Validate checks every row for field correctness, duplicate hosts, and
// resolves location codes and parent device hosts against the database.
// It never aborts early — all rows are validated and issues collected.
func (s *ImportService) Validate(ctx context.Context, rows []ImportRow) (*ImportPreview, error) {
	preview := &ImportPreview{
		TotalRows: len(rows),
		Rows:      make([]RowValidation, 0, len(rows)),
	}

	// Track hosts seen in this batch to catch intra-file duplicates.
	seenHosts := make(map[string]int) // host -> first line number

	for i := range rows {
		rv := s.validateRow(ctx, &rows[i], seenHosts)
		preview.Rows = append(preview.Rows, rv)

		if !rv.Valid {
			preview.Errors++
		}
		if len(rv.Warnings) > 0 {
			preview.Warnings++
		}
		if rv.IsDuplicate {
			preview.Duplicates++
		}
		if rv.Valid {
			preview.Valid++
		}
	}

	return preview, nil
}

// validateRow performs all checks for a single row and mutates row to fill
// resolved IDs.
func (s *ImportService) validateRow(ctx context.Context, row *ImportRow, seen map[string]int) RowValidation {
	rv := RowValidation{Row: *row, Valid: true}

	// --- Required fields ---
	if row.Name == "" {
		rv.Errors = append(rv.Errors, "name is required")
		rv.Valid = false
	}
	if row.Host == "" {
		rv.Errors = append(rv.Errors, "host is required")
		rv.Valid = false
	}

	// --- Host format ---
	if row.Host != "" && !isValidHost(row.Host) {
		rv.Warnings = append(rv.Warnings, fmt.Sprintf("host %q may not be a valid IP address, hostname, or URL", row.Host))
	}

	// --- Protocol default ---
	if row.Protocol == "" {
		row.Protocol = "ping"
		rv.Row.Protocol = "ping"
	}

	// --- Port ---
	if row.Port < 0 {
		rv.Errors = append(rv.Errors, "port must be positive")
		rv.Valid = false
	}

	// --- Intra-file duplicate ---
	if row.Host != "" {
		if firstLine, dup := seen[row.Host]; dup {
			rv.Warnings = append(rv.Warnings, fmt.Sprintf("duplicate host in CSV (first seen on line %d)", firstLine))
			rv.IsDuplicate = true
		} else {
			seen[row.Host] = row.LineNumber
		}
	}

	// --- DB duplicate check ---
	if row.Host != "" {
		var existingID int64
		err := s.db.QueryRow(ctx,
			"SELECT id FROM devices WHERE ip_address = $1 LIMIT 1", row.Host,
		).Scan(&existingID)
		if err == nil {
			rv.IsDuplicate = true
			rv.Warnings = append(rv.Warnings, fmt.Sprintf("host already exists in database (device id %d)", existingID))
		} else if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("csv import: error checking host duplicate", "host", row.Host, "err", err)
		}
	}

	// --- Location resolution ---
	if row.LocationCode != "" {
		var locID int64
		err := s.db.QueryRow(ctx,
			"SELECT id FROM locations WHERE code = $1 LIMIT 1", row.LocationCode,
		).Scan(&locID)
		if err == nil {
			row.ResolvedLocationID = &locID
			rv.Row.ResolvedLocationID = &locID
		} else if errors.Is(err, pgx.ErrNoRows) {
			rv.Warnings = append(rv.Warnings, fmt.Sprintf("location code %q not found", row.LocationCode))
		} else {
			slog.Warn("csv import: error resolving location", "code", row.LocationCode, "err", err)
		}
	}

	// --- Parent device resolution ---
	if row.ParentDeviceHost != "" {
		var parentID int64
		err := s.db.QueryRow(ctx,
			"SELECT id FROM devices WHERE ip_address = $1 LIMIT 1", row.ParentDeviceHost,
		).Scan(&parentID)
		if err == nil {
			row.ResolvedParentID = &parentID
			rv.Row.ResolvedParentID = &parentID
		} else if errors.Is(err, pgx.ErrNoRows) {
			rv.Warnings = append(rv.Warnings, fmt.Sprintf("parent device host %q not found", row.ParentDeviceHost))
		} else {
			slog.Warn("csv import: error resolving parent device", "host", row.ParentDeviceHost, "err", err)
		}
	}

	// --- MAC address format ---
	if row.MACAddress != "" && !macRegex.MatchString(row.MACAddress) {
		rv.Errors = append(rv.Errors, fmt.Sprintf("invalid MAC address format %q (expected XX:XX:XX:XX:XX:XX)", row.MACAddress))
		rv.Valid = false
	}

	return rv
}

// isValidHost returns true if s is a valid IPv4 address, IPv6 address,
// hostname, or URL.
func isValidHost(s string) bool {
	// Try IP first (covers both v4 and v6).
	if net.ParseIP(s) != nil {
		return true
	}
	// Try as a URL (scheme://host).
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		return true
	}
	// Accept as a hostname if it contains only valid characters.
	// RFC-952 / RFC-1123 hostname: alphanumeric, hyphens, dots.
	hostnameRe := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?$`)
	return hostnameRe.MatchString(s)
}

// ---------- Execute ----------

// Execute inserts all provided rows into the devices table inside a single
// transaction and creates a default sensor for each device. If any insert
// fails the entire transaction is rolled back.
func (s *ImportService) Execute(ctx context.Context, rows []ImportRow) (*ImportResult, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	result := &ImportResult{}

	for _, row := range rows {
		var deviceID int64
		err = tx.QueryRow(ctx,
			`INSERT INTO devices
				(name, ip_address, protocol, port, enabled, status,
				 location_id, parent_device_id, mac_address, asset_tag,
				 device_category, notes)
			VALUES ($1, $2, $3, $4, TRUE, 'unknown',
				$5, $6, $7, $8, $9, $10)
			RETURNING id`,
			row.Name,
			row.Host,
			row.Protocol,
			row.Port,
			row.ResolvedLocationID,
			row.ResolvedParentID,
			row.MACAddress,
			row.AssetTag,
			row.DeviceCategory,
			row.Notes,
		).Scan(&deviceID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("line %d (%s): %v", row.LineNumber, row.Name, err))
			return result, fmt.Errorf("insert device %q (line %d): %w", row.Name, row.LineNumber, err)
		}
		result.DevicesCreated++

		// Create a default sensor for the device.
		sensorName := fmt.Sprintf("%s - %s", row.Name, row.Protocol)
		_, err = tx.Exec(ctx,
			`INSERT INTO sensors (device_id, name, type, interval, config, enabled)
			VALUES ($1, $2, $3, 30, '{}', TRUE)`,
			deviceID, sensorName, row.Protocol,
		)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("line %d sensor: %v", row.LineNumber, err))
			return result, fmt.Errorf("insert sensor for device %q (line %d): %w", row.Name, row.LineNumber, err)
		}
		result.SensorsCreated++
	}

	if err = tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit transaction: %w", err)
	}

	slog.Info("csv import complete",
		"devices_created", result.DevicesCreated,
		"sensors_created", result.SensorsCreated,
	)
	return result, nil
}
