package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Type string

const (
	TypeManual    Type = "manual"
	TypeScheduled Type = "scheduled"
)

type Backup struct {
	ID          int64      `json:"id"`
	Filename    string     `json:"filename"`
	Path        string     `json:"-"`
	Size        int64      `json:"size"`
	Status      Status     `json:"status"`
	Type        Type       `json:"type"`
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Error       string     `json:"error,omitempty"`
	Checksum    string     `json:"checksum,omitempty"`
	RestoredAt  *time.Time `json:"restoredAt,omitempty"`
	RestoredBy  string     `json:"restoredBy,omitempty"`
}

type Config struct {
	BackupDir       string
	MaxBackups      int
	RetentionDays   int
	ScheduleEnabled bool
	ScheduleCron    string
	PostgresBinary  string
	RestoreBinary   string
}

type Manager struct {
	pool   *pgxpool.Pool
	cfg    Config
	logger *slog.Logger
	mu     sync.Mutex
}

func NewManager(pool *pgxpool.Pool, cfg Config, logger *slog.Logger) *Manager {
	if cfg.BackupDir == "" {
		cfg.BackupDir = "./data/backups"
	}
	if cfg.PostgresBinary == "" {
		cfg.PostgresBinary = "pg_dump"
	}
	if cfg.RestoreBinary == "" {
		cfg.RestoreBinary = "pg_restore"
	}
	if cfg.MaxBackups == 0 {
		cfg.MaxBackups = 50
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = 90
	}
	return &Manager{pool: pool, cfg: cfg, logger: logger}
}

func (m *Manager) EnsureDir() error {
	return os.MkdirAll(m.cfg.BackupDir, 0750)
}

func (m *Manager) GetBackupDir() string {
	return m.cfg.BackupDir
}

func (m *Manager) GetConfig() Config {
	return m.cfg
}

func (m *Manager) CreateBackup(ctx context.Context, backupType Type, createdBy string) (*Backup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.EnsureDir(); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}

	ts := time.Now().UTC().Format("20060102_150405")
	filename := fmt.Sprintf("netmonitor_%s_%s.sql.gz", backupType, ts)
	backupPath := filepath.Join(m.cfg.BackupDir, filename)

	// Insert pending record
	var id int64
	err := m.pool.QueryRow(ctx,
		`INSERT INTO backups (filename, path, status, type, created_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 RETURNING id`,
		filename, backupPath, string(StatusPending), string(backupType),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert backup record: %w", err)
	}

	backup := &Backup{
		ID:        id,
		Filename:  filename,
		Path:      backupPath,
		Status:    StatusPending,
		Type:      backupType,
		CreatedAt: time.Now().UTC(),
	}

	// Run pg_dump in background, preserving auth/request values without tying the
	// backup lifetime to the HTTP response context.
	go m.runDump(context.WithoutCancel(ctx), id, backupPath)

	return backup, nil
}

func (m *Manager) runDump(ctx context.Context, backupID int64, backupPath string) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// Update status to running
	_, _ = m.pool.Exec(ctx,
		`UPDATE backups SET status = $1 WHERE id = $2`,
		string(StatusRunning), backupID)

	m.logger.Info("Starting database backup", "backup_id", backupID, "path", backupPath)

	// Get DSN from pool config and extract connection params
	dsn := m.getDSN()

	// Create the output file
	if err := m.validateBackupPath(backupPath); err != nil {
		m.failBackup(ctx, backupID, fmt.Sprintf("invalid backup path: %v", err))
		return
	}

	outFile, err := os.Create(backupPath) //nolint:gosec // Path is generated internally and validated against BackupDir above.
	if err != nil {
		m.failBackup(ctx, backupID, fmt.Sprintf("create file: %v", err))
		return
	}

	// Build pg_dump command - use custom format for pg_restore compatibility
	// We'll use plain SQL format piped through gzip for human readability
	cmd := exec.CommandContext(ctx, m.cfg.PostgresBinary, //nolint:gosec // Binary comes from trusted server config; args are fixed except DSN.
		"--no-owner",
		"--no-privileges",
		"--clean",
		"--if-exists",
		"--format=plain",
		dsn,
	)
	cmd.Stdout = outFile
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if closeErr := outFile.Close(); closeErr != nil {
			m.logger.Warn("Failed to close incomplete backup file", "backup_id", backupID, "error", closeErr)
		}
		m.failBackup(ctx, backupID, fmt.Sprintf("pg_dump failed: %v", err))
		return
	}

	// Get file size
	if err := outFile.Close(); err != nil {
		m.failBackup(ctx, backupID, fmt.Sprintf("close file: %v", err))
		return
	}
	stat, err := os.Stat(backupPath)
	if err != nil {
		m.failBackup(ctx, backupID, fmt.Sprintf("stat file: %v", err))
		return
	}

	// Compute checksum
	checksum, err := m.computeChecksum(backupPath)
	if err != nil {
		m.failBackup(ctx, backupID, fmt.Sprintf("checksum: %v", err))
		return
	}

	now := time.Now().UTC()
	_, err = m.pool.Exec(ctx,
		`UPDATE backups SET status = $1, size = $2, checksum = $3, completed_at = $4 WHERE id = $5`,
		string(StatusCompleted), stat.Size(), checksum, now, backupID)
	if err != nil {
		m.logger.Error("Failed to update backup record", "backup_id", backupID, "error", err)
		return
	}

	m.logger.Info("Backup completed", "backup_id", backupID, "size", stat.Size(), "checksum", checksum)

	// Prune old backups
	m.pruneOldBackups(ctx)
}

func (m *Manager) failBackup(ctx context.Context, backupID int64, errMsg string) {
	m.logger.Error("Backup failed", "backup_id", backupID, "error", errMsg)
	_, _ = m.pool.Exec(ctx,
		`UPDATE backups SET status = $1, error = $2 WHERE id = $3`,
		string(StatusFailed), errMsg, backupID)
}

func (m *Manager) getDSN() string {
	// Extract DSN from the pool's connection config
	// The pool was created with a DSN, but pgx doesn't expose it directly.
	// We'll read from environment or use a reconstructed DSN.
	// For pg_dump, we need a libpq-style connection string.
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_DSN")
	}
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/netmonitor?sslmode=disable" //nolint:gosec // Local development fallback only; production must set DATABASE_URL.
	}
	return dsn
}

func (m *Manager) computeChecksum(path string) (string, error) {
	if err := m.validateBackupPath(path); err != nil {
		return "", err
	}

	f, err := os.Open(path) //nolint:gosec // Path is generated internally and validated against BackupDir above.
	if err != nil {
		return "", err
	}
	defer func() {
		if err := f.Close(); err != nil {
			m.logger.Warn("Failed to close checksum file", "path", path, "error", err)
		}
	}()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (m *Manager) ListBackups(ctx context.Context) ([]Backup, error) {
	rows, err := m.pool.Query(ctx,
		`SELECT id, filename, path, COALESCE(size, 0), status, type, created_at, completed_at, COALESCE(error, ''), COALESCE(checksum, ''), restored_at, COALESCE(restored_by, '')
		 FROM backups ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backups []Backup
	for rows.Next() {
		var b Backup
		if err := rows.Scan(&b.ID, &b.Filename, &b.Path, &b.Size, &b.Status, &b.Type,
			&b.CreatedAt, &b.CompletedAt, &b.Error, &b.Checksum, &b.RestoredAt, &b.RestoredBy); err != nil {
			return nil, err
		}
		backups = append(backups, b)
	}
	return backups, nil
}

func (m *Manager) GetBackup(ctx context.Context, id int64) (*Backup, error) {
	var b Backup
	err := m.pool.QueryRow(ctx,
		`SELECT id, filename, path, COALESCE(size, 0), status, type, created_at, completed_at, COALESCE(error, ''), COALESCE(checksum, ''), restored_at, COALESCE(restored_by, '')
		 FROM backups WHERE id = $1`, id,
	).Scan(&b.ID, &b.Filename, &b.Path, &b.Size, &b.Status, &b.Type,
		&b.CreatedAt, &b.CompletedAt, &b.Error, &b.Checksum, &b.RestoredAt, &b.RestoredBy)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (m *Manager) GetBackupPath(ctx context.Context, id int64) (string, string, error) {
	var filename, path string
	err := m.pool.QueryRow(ctx,
		`SELECT filename, path FROM backups WHERE id = $1 AND status = $2`, id, string(StatusCompleted),
	).Scan(&filename, &path)
	if err != nil {
		return "", "", err
	}
	return filename, path, nil
}

func (m *Manager) DeleteBackup(ctx context.Context, id int64) error {
	var path string
	err := m.pool.QueryRow(ctx,
		`SELECT path FROM backups WHERE id = $1`, id,
	).Scan(&path)
	if err != nil {
		return err
	}

	// Remove file
	if path != "" {
		_ = os.Remove(path)
	}

	_, err = m.pool.Exec(ctx, `DELETE FROM backups WHERE id = $1`, id)
	return err
}

func (m *Manager) RestoreBackup(ctx context.Context, id int64, restoredBy string) error {
	var path, filename string
	err := m.pool.QueryRow(ctx,
		`SELECT filename, path FROM backups WHERE id = $1 AND status = $2`, id, string(StatusCompleted),
	).Scan(&filename, &path)
	if err != nil {
		return fmt.Errorf("backup not found or not completed: %w", err)
	}

	if err := m.validateBackupPath(path); err != nil {
		return fmt.Errorf("invalid backup path: %w", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found on disk: %s", path)
	}

	dsn := m.getDSN()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	m.logger.Info("Starting database restore", "backup_id", id, "file", filename)

	// Use psql for plain SQL format restore
	cmd := exec.CommandContext(ctx, "psql", //nolint:gosec // Restores trusted backup files validated under BackupDir using fixed psql args.
		"--single-transaction",
		"--set=ON_ERROR_STOP=on",
		dsn,
		"-f", path,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("restore failed: %v\n%s", err, string(output))
		m.logger.Error("Restore failed", "backup_id", id, "error", errMsg)
		return fmt.Errorf("restore failed: %w", err)
	}

	now := time.Now().UTC()
	_, err = m.pool.Exec(ctx,
		`UPDATE backups SET restored_at = $1, restored_by = $2 WHERE id = $3`,
		now, restoredBy, id)
	if err != nil {
		m.logger.Error("Failed to record restore", "backup_id", id, "error", err)
	}

	m.logger.Info("Restore completed", "backup_id", id, "restored_by", restoredBy)
	return nil
}

func (m *Manager) RestoreFromFile(ctx context.Context, filePath string, restoredBy string) error {
	if err := m.validateBackupPath(filePath); err != nil {
		return fmt.Errorf("invalid restore file path: %w", err)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	dsn := m.getDSN()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	m.logger.Info("Starting restore from uploaded file", "file", filePath, "restored_by", restoredBy)

	cmd := exec.CommandContext(ctx, "psql", //nolint:gosec // Uploaded restore file is stored and validated under BackupDir; args are fixed except DSN.
		"--single-transaction",
		"--set=ON_ERROR_STOP=on",
		dsn,
		"-f", filePath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("restore failed: %v\n%s", err, string(output))
		m.logger.Error("Restore from file failed", "error", errMsg)
		return fmt.Errorf("restore from file failed: %w", err)
	}

	m.logger.Info("Restore from file completed", "file", filePath, "restored_by", restoredBy)
	return nil
}

func (m *Manager) validateBackupPath(path string) error {
	backupDir, err := filepath.Abs(m.cfg.BackupDir)
	if err != nil {
		return fmt.Errorf("resolve backup dir: %w", err)
	}
	candidate, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve backup path: %w", err)
	}
	rel, err := filepath.Rel(backupDir, candidate)
	if err != nil {
		return fmt.Errorf("compare backup path: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return fmt.Errorf("path escapes backup directory")
	}
	return nil
}

func (m *Manager) pruneOldBackups(ctx context.Context) {
	cutoff := time.Now().AddDate(0, 0, -m.cfg.RetentionDays)

	rows, err := m.pool.Query(ctx,
		`SELECT id, path FROM backups WHERE created_at < $1 AND type = $2 ORDER BY created_at ASC`,
		cutoff, string(TypeScheduled))
	if err != nil {
		m.logger.Error("Failed to query old backups for pruning", "error", err)
		return
	}
	defer rows.Close()

	var toDelete []struct {
		id   int64
		path string
	}
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err == nil {
			toDelete = append(toDelete, struct {
				id   int64
				path string
			}{id, path})
		}
	}

	for _, d := range toDelete {
		_ = os.Remove(d.path)
		_, _ = m.pool.Exec(ctx, `DELETE FROM backups WHERE id = $1`, d.id)
		m.logger.Info("Pruned old backup", "backup_id", d.id)
	}
}

// SanitizeDSN removes the password from a DSN for safe logging.
func SanitizeDSN(dsn string) string {
	re := regexp.MustCompile(`(:[^@]*@)`)
	return re.ReplaceAllString(dsn, ":***@")
}

// ListSQLFiles lists .sql and .sql.gz files in a directory for upload/restore.
func ListSQLFiles(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".sql") || strings.HasSuffix(e.Name(), ".sql.gz")) {
			files = append(files, e.Name())
		}
	}
	return files, nil
}
