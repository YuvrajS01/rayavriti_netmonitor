package handlers

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/backup"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

const (
	uploadMaxBytes        = 500 << 20
	uploadMultipartMemory = 32 << 20
)

type BackupHandler struct {
	manager *backup.Manager
}

func NewBackupHandler(manager *backup.Manager) *BackupHandler {
	return &BackupHandler{manager: manager}
}

func (h *BackupHandler) List(w http.ResponseWriter, r *http.Request) {
	backups, err := h.manager.ListBackups(r.Context())
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	if backups == nil {
		backups = []backup.Backup{}
	}
	httputil.SendOK(w, backups)
}

func (h *BackupHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid backup id")
		return
	}
	b, err := h.manager.GetBackup(r.Context(), id)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "backup not found")
		return
	}
	httputil.SendOK(w, b)
}

func (h *BackupHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	createdBy := "admin"
	if claims != nil {
		createdBy = claims.Username
	}

	b, err := h.manager.CreateBackup(r.Context(), backup.TypeManual, createdBy)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendCreated(w, b)
}

func (h *BackupHandler) Download(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid backup id")
		return
	}

	filename, path, err := h.manager.GetBackupPath(r.Context(), id)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "backup not found")
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) { //nolint:gosec // Path comes from completed backup metadata resolved by manager.
		httputil.SendError(w, http.StatusNotFound, "backup file not found on disk")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path) //nolint:gosec // Path comes from completed backup metadata resolved by manager.
}

func (h *BackupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid backup id")
		return
	}

	if err := h.manager.DeleteBackup(r.Context(), id); err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, map[string]string{"message": "backup deleted"})
}

func (h *BackupHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid backup id")
		return
	}

	claims := auth.GetClaims(r.Context())
	restoredBy := "admin"
	if claims != nil {
		restoredBy = claims.Username
	}

	if err := h.manager.RestoreBackup(r.Context(), id, restoredBy); err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, map[string]string{"message": "restore completed successfully"})
}

func (h *BackupHandler) Upload(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	restoredBy := "admin"
	if claims != nil {
		restoredBy = claims.Username
	}

	r.Body = http.MaxBytesReader(w, r.Body, uploadMaxBytes)
	if err := r.ParseMultipartForm(uploadMultipartMemory); err != nil { //nolint:gosec // Request body is capped by MaxBytesReader above.
		httputil.SendError(w, http.StatusBadRequest, "failed to parse upload: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "no file provided")
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Default().Warn("failed to close uploaded backup file", "error", err)
		}
	}()

	// Validate file extension
	ext := filepath.Ext(header.Filename)
	if ext != ".sql" && ext != ".gz" && ext != ".backup" {
		httputil.SendError(w, http.StatusBadRequest, "unsupported file type. Allowed: .sql, .gz, .backup")
		return
	}

	// Save to temp file
	tmpDir := h.manager.GetBackupDir()
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to create temp directory")
		return
	}

	tmpFile, err := os.CreateTemp(tmpDir, "upload-*.sql")
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to create temp file")
		return
	}
	tmpFileClosed := false
	defer func() {
		if !tmpFileClosed {
			if err := tmpFile.Close(); err != nil {
				slog.Default().Warn("failed to close temporary backup file", "path", tmpFile.Name(), "error", err)
			}
		}
		if err := os.Remove(tmpFile.Name()); err != nil && !os.IsNotExist(err) {
			slog.Default().Warn("failed to remove temporary backup file", "path", tmpFile.Name(), "error", err)
		}
	}()

	if _, err := io.Copy(tmpFile, file); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to save uploaded file")
		return
	}
	if err := tmpFile.Close(); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to finalize uploaded file")
		return
	}
	tmpFileClosed = true

	// Run restore
	if err := h.manager.RestoreFromFile(r.Context(), tmpFile.Name(), restoredBy); err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, map[string]string{"message": "restore from uploaded file completed successfully"})
}

func (h *BackupHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	dir := h.manager.GetBackupDir()
	files, err := backup.ListSQLFiles(dir)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	if files == nil {
		files = []string{}
	}
	httputil.SendOK(w, files)
}

func (h *BackupHandler) Config(w http.ResponseWriter, r *http.Request) {
	cfg := h.manager.GetConfig()
	httputil.SendOK(w, map[string]any{
		"backupDir":       cfg.BackupDir,
		"maxBackups":      cfg.MaxBackups,
		"retentionDays":   cfg.RetentionDays,
		"scheduleEnabled": cfg.ScheduleEnabled,
		"scheduleCron":    cfg.ScheduleCron,
	})
}

// Ensure time is used for potential future scheduling features
var _ = time.Now
