package health

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"
)

const (
	statusOK    = "ok"
	statusError = "error"
	statusUp    = "up"
	statusDown  = "down"
)

type Indicator interface {
	Name() string
	IsHealthy(ctx context.Context) bool
}

type IndicatorStatus struct {
	Status string `json:"status"`
}

type LivenessResponse struct {
	Status string                     `json:"status"`
	Info   map[string]IndicatorStatus `json:"info"`
	Error  map[string]IndicatorStatus `json:"error"`
}

type HealthManager struct {
	indicators []Indicator
}

func NewHealthManager(indicators ...Indicator) *HealthManager {
	return &HealthManager{indicators: indicators}
}

func (m *HealthManager) RegisterRoutes(e *echo.Echo) {
	e.GET("/health/liveness", m.Liveness)
}

func (m *HealthManager) Liveness(c *echo.Context) error {
	response := LivenessResponse{
		Status: statusOK,
		Info:   map[string]IndicatorStatus{},
		Error:  map[string]IndicatorStatus{},
	}

	for _, indicator := range m.indicators {
		if indicator.IsHealthy(c.Request().Context()) {
			response.Info[indicator.Name()] = IndicatorStatus{Status: statusUp}
			continue
		}
		response.Status = statusError
		response.Error[indicator.Name()] = IndicatorStatus{Status: statusDown}
	}

	statusCode := http.StatusOK
	if response.Status == statusError {
		statusCode = http.StatusServiceUnavailable
	}

	return c.JSON(statusCode, response)
}
