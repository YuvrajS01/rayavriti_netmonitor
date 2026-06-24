package handlers

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/reports"
)

type ReportGenHandler struct {
	generator *reports.Generator
	pool      *pgxpool.Pool
}

func NewReportGenHandler(db database.Database, outputDir string) *ReportGenHandler {
	pp, ok := db.(database.PoolProvider)
	if !ok || pp.Pool() == nil {
		slog.Warn("ReportGenHandler: database does not provide a pool, report generation will be unavailable")
		return &ReportGenHandler{}
	}
	pool := pp.Pool()
	generator := reports.NewGenerator(pool, outputDir)
	return &ReportGenHandler{generator: generator, pool: pool}
}

func (h *ReportGenHandler) Generator() *reports.Generator {
	return h.generator
}

func (h *ReportGenHandler) GenerateReport(w http.ResponseWriter, r *http.Request) {
	if h.generator == nil {
		httputil.SendError(w, http.StatusNotImplemented, "report generation unavailable")
		return
	}

	var body struct {
		ReportType string `json:"reportType"`
		Format     string `json:"format"`
		PeriodFrom string `json:"periodFrom"`
		PeriodTo   string `json:"periodTo"`
		Title      string `json:"title"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.ReportType == "" {
		httputil.SendError(w, http.StatusBadRequest, "reportType is required")
		return
	}

	var from, to time.Time
	if body.PeriodFrom != "" {
		from, _ = time.Parse("2006-01-02", body.PeriodFrom)
	}
	if body.PeriodTo != "" {
		to, _ = time.Parse("2006-01-02", body.PeriodTo)
	}

	var genBy string
	if claims := auth.GetClaims(r.Context()); claims != nil {
		genBy = fmt.Sprintf("user:%d", claims.UserID)
	} else {
		genBy = "api"
	}

	result, err := h.generator.Generate(r.Context(), reports.GenerateRequest{
		ReportType:  body.ReportType,
		Title:       body.Title,
		Format:      body.Format,
		PeriodFrom:  from,
		PeriodTo:    to,
		GeneratedBy: genBy,
	})
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "generation failed: "+err.Error())
		return
	}
	httputil.SendCreated(w, result)
}

func (h *ReportGenHandler) DownloadReport(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "report service unavailable")
		return
	}

	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var filePath, title, format string
	err = h.pool.QueryRow(r.Context(),
		`SELECT file_path, title, format FROM generated_reports WHERE id=$1`, id,
	).Scan(&filePath, &title, &format)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "report not found")
		return
	}

	f, err := os.Open(filePath) //nolint:gosec // filePath from database, not user input
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "report file not found on disk")
		return
	}
	defer func() { _ = f.Close() }()

	contentType := "application/octet-stream"
	switch format {
	case "csv":
		contentType = "text/csv"
	case "html":
		contentType = "text/html"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.%s"`, title, format))
	if _, err := io.Copy(w, f); err != nil {
		slog.Error("Failed to serve report file", "id", id, "error", err)
	}
}

func (h *ReportGenHandler) RunScheduledReport(w http.ResponseWriter, r *http.Request) {
	if h.generator == nil {
		httputil.SendError(w, http.StatusNotImplemented, "report generation unavailable")
		return
	}

	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var name, reportType, format, lookback string
	var customFrom, customTo *time.Time
	err = h.pool.QueryRow(r.Context(),
		`SELECT name, report_type, format, lookback_period, custom_from, custom_to
		 FROM scheduled_reports WHERE id=$1 AND enabled=true`, id,
	).Scan(&name, &reportType, &format, &lookback, &customFrom, &customTo)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "scheduled report not found")
		return
	}

	periodTo := time.Now()
	periodFrom := periodTo.AddDate(0, 0, -7)
	if customFrom != nil && customTo != nil {
		periodFrom = *customFrom
		periodTo = *customTo
	} else {
		switch lookback {
		case "1d":
			periodFrom = periodTo.AddDate(0, 0, -1)
		case "30d":
			periodFrom = periodTo.AddDate(0, 0, -30)
		case "90d":
			periodFrom = periodTo.AddDate(0, 0, -90)
		case "1y":
			periodFrom = periodTo.AddDate(-1, 0, 0)
		}
	}

	var genBy string
	if claims := auth.GetClaims(r.Context()); claims != nil {
		genBy = fmt.Sprintf("user:%d", claims.UserID)
	} else {
		genBy = "api"
	}

	result, err := h.generator.Generate(r.Context(), reports.GenerateRequest{
		ReportType:        reportType,
		Title:             name,
		Format:            format,
		PeriodFrom:        periodFrom,
		PeriodTo:          periodTo,
		GeneratedBy:       genBy,
		ScheduledReportID: &id,
	})
	if err != nil {
		_, _ = h.pool.Exec(r.Context(),
			`UPDATE scheduled_reports SET last_run_at=NOW(), last_run_status='failed' WHERE id=$1`, id)
		httputil.SendError(w, http.StatusInternalServerError, "generation failed: "+err.Error())
		return
	}

	_, _ = h.pool.Exec(r.Context(),
		`UPDATE scheduled_reports SET last_run_at=NOW(), last_run_status='success' WHERE id=$1`, id)

	httputil.SendOK(w, result)
}

func (h *ReportGenHandler) ListGenerated(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "report service unavailable")
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, report_type, title, format, file_path, file_size_bytes, period_from, period_to, generated_by, generated_at
		 FROM generated_reports ORDER BY generated_at DESC LIMIT $1`, limit)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	reports := []map[string]any{}
	for rows.Next() {
		var id int64
		var rType, title, format, filePath string
		var fileSize *int64
		var periodFrom, periodTo *time.Time
		var genBy *string
		var genAt time.Time
		if err := rows.Scan(&id, &rType, &title, &format, &filePath, &fileSize, &periodFrom, &periodTo, &genBy, &genAt); err != nil {
			continue
		}
		reports = append(reports, map[string]any{
			"id":          id,
			"reportType":  rType,
			"title":       title,
			"format":      format,
			"filePath":    filePath,
			"fileSize":    fileSize,
			"periodFrom":  periodFrom,
			"periodTo":    periodTo,
			"generatedBy": genBy,
			"generatedAt": genAt,
		})
	}
	httputil.SendOK(w, reports)
}
