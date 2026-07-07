package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/backup"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
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
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
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
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
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

	if _, err := os.Stat(path); os.IsNotExist(err) {
		httputil.SendError(w, http.StatusNotFound, "backup file not found on disk")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path)
}

func (h *BackupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid backup id")
		return
	}

	if err := h.manager.DeleteBackup(r.Context(), id); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
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
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
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

	// Parse multipart form (max 500MB)
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "failed to parse upload: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "no file provided")
		return
	}
	defer file.Close()

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
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()

	if _, err := io.Copy(tmpFile, file); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to save uploaded file")
		return
	}
	tmpFile.Close()

	// Run restore
	if err := h.manager.RestoreFromFile(r.Context(), tmpFile.Name(), restoredBy); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"message": "restore from uploaded file completed successfully"})
}

func (h *BackupHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	dir := h.manager.GetBackupDir()
	files, err := backup.ListSQLFiles(dir)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
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
