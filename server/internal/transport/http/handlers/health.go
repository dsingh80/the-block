package handlers

import (
	"net/http"

	"github.com/dsingh80/the-block/server/internal/transport/dto"
	"github.com/dsingh80/the-block/server/internal/transport/httputil"
	"github.com/dsingh80/the-block/server/internal/usecase/health"
)

// Health serves GET /healthcheck: liveness/readiness for an orchestrator
// (guidelines/06-backend-architecture.md, "API design").
type Health struct {
	redis    health.Pinger
	postgres health.Pinger
}

func NewHealth(redis, postgres health.Pinger) *Health {
	return &Health{redis: redis, postgres: postgres}
}

func (h *Health) Check(w http.ResponseWriter, r *http.Request) {
	redisErr := h.redis.Ping(r.Context())
	postgresErr := h.postgres.Ping(r.Context())

	body := dto.HealthCheck{Status: "ok", Redis: pingStatus(redisErr), Postgres: pingStatus(postgresErr)}
	status := http.StatusOK
	if redisErr != nil || postgresErr != nil {
		body.Status = "degraded"
		status = http.StatusServiceUnavailable
	}
	httputil.WriteJSON(w, status, body)
}

func pingStatus(err error) string {
	if err != nil {
		return "unreachable"
	}
	return "ok"
}
