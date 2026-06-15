package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type InsightHandler struct{ db database.Database }

func NewInsightHandler(db database.Database) *InsightHandler { return &InsightHandler{db: db} }

type InsightsResponse struct {
	GeneratedAt        string               `json:"generatedAt"`
	NetworkScore       int                  `json:"networkScore"`
	HealthDistribution HealthDistribution   `json:"healthDistribution"`
	TopRisks           []TopRiskDevice      `json:"topRisks"`
	Health             []DeviceHealthDetail `json:"health"`
	Insights           []InsightItem        `json:"insights"`
}

type HealthDistribution struct {
	Critical int `json:"critical"`
	Risk     int `json:"risk"`
	Watch    int `json:"watch"`
	Healthy  int `json:"healthy"`
}

type TopRiskDevice struct {
	DeviceID     int64   `json:"deviceId"`
	DeviceName   string  `json:"deviceName"`
	Score        float64 `json:"score"`
	Label        string  `json:"label"`
	Trend        string  `json:"trend"`
	TrendDelta   float64 `json:"trendDelta"`
	PrimaryIssue string  `json:"primaryIssue"`
}

type DeviceHealthDetail struct {
	DeviceID            int64             `json:"deviceId"`
	DeviceName          string            `json:"deviceName"`
	Score               float64           `json:"score"`
	Label               string            `json:"label"`
	AvailabilityPercent float64           `json:"availabilityPercent"`
	AvgResponseMs       int               `json:"avgResponseMs"`
	ActiveAlerts        int               `json:"activeAlerts"`
	OpenPorts           int               `json:"openPorts"`
	Samples             int               `json:"samples"`
	Factors             any               `json:"factors"`
	Trend               string            `json:"trend"`
	TrendDelta          float64           `json:"trendDelta"`
	Issues              []healthIssueJSON `json:"issues"`
}

type healthIssueJSON struct {
	Severity string `json:"severity"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

type InsightItem struct {
	DeviceID   int64   `json:"deviceId"`
	DeviceName string  `json:"deviceName"`
	Score      float64 `json:"score"`
	Status     string  `json:"status"`
	Type       string  `json:"type"`
	Severity   string  `json:"severity"`
	Title      string  `json:"title"`
	Message    string  `json:"message"`
}

func (h *InsightHandler) Current(w http.ResponseWriter, r *http.Request) {
	scores, err := h.db.GetHealthScores(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}

	if len(scores) == 0 {
		httputil.SendOK(w, InsightsResponse{
			GeneratedAt:        time.Now().Format(time.RFC3339),
			NetworkScore:       0,
			HealthDistribution: HealthDistribution{},
			TopRisks:           []TopRiskDevice{},
			Health:             []DeviceHealthDetail{},
			Insights:           []InsightItem{},
		})
		return
	}

	devices, err := h.db.GetDevices(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	deviceMap := make(map[int64]string, len(devices))
	for _, d := range devices {
		deviceMap[d.ID] = d.Name
	}

	// Build health details from persisted scores.
	var health []DeviceHealthDetail
	var topRisks []TopRiskDevice
	var insights []InsightItem
	dist := HealthDistribution{}
	totalScore := 0.0

	for _, s := range scores {
		name := deviceMap[s.DeviceID]
		totalScore += s.Score

		switch s.Label {
		case "critical":
			dist.Critical++
		case "risk":
			dist.Risk++
		case "watch":
			dist.Watch++
		default:
			dist.Healthy++
		}

		var factors any
		if s.Factors != nil {
			_ = json.Unmarshal(s.Factors, &factors)
		}
		var issues []healthIssueJSON
		if s.Issues != nil {
			_ = json.Unmarshal(s.Issues, &issues)
		}

		detail := DeviceHealthDetail{
			DeviceID:   s.DeviceID,
			DeviceName: name,
			Score:      s.Score,
			Label:      s.Label,
			Factors:    factors,
			Trend:      s.Trend,
			TrendDelta: s.TrendDelta,
			Issues:     issues,
		}
		health = append(health, detail)

		if s.Label == "critical" || s.Label == "risk" {
			primaryIssue := "No issues"
			if len(issues) > 0 {
				primaryIssue = issues[0].Message
			}
			topRisks = append(topRisks, TopRiskDevice{
				DeviceID:     s.DeviceID,
				DeviceName:   name,
				Score:        s.Score,
				Label:        s.Label,
				Trend:        s.Trend,
				TrendDelta:   s.TrendDelta,
				PrimaryIssue: primaryIssue,
			})
			insights = append(insights, InsightItem{
				DeviceID:   s.DeviceID,
				DeviceName: name,
				Score:      s.Score,
				Status:     s.Label,
				Type:       "health",
				Severity:   s.Label,
				Title:      name + " — " + strconv.Itoa(int(s.Score)) + "%",
				Message:    primaryIssue,
			})
		}
	}

	// Trim topRisks to 5.
	if len(topRisks) > 5 {
		topRisks = topRisks[:5]
	}

	networkScore := 0
	if len(scores) > 0 {
		networkScore = int(totalScore / float64(len(scores)))
	}

	httputil.SendOK(w, InsightsResponse{
		GeneratedAt:        time.Now().Format(time.RFC3339),
		NetworkScore:       networkScore,
		HealthDistribution: dist,
		TopRisks:           topRisks,
		Health:             health,
		Insights:           insights,
	})
}

func (h *InsightHandler) History(w http.ResponseWriter, r *http.Request) {
	hoursStr := r.URL.Query().Get("hours")
	hours := 12
	if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
		hours = h
	}

	points, err := h.db.GetNetworkHealthHistory(r.Context(), hours)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}

	type historyResponse struct {
		GeneratedAt string                      `json:"generatedAt"`
		Hours       int                         `json:"hours"`
		Points      []models.HealthHistoryPoint `json:"points"`
	}

	httputil.SendOK(w, historyResponse{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Hours:       hours,
		Points:      points,
	})
}
