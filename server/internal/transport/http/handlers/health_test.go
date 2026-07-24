package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsingh80/the-block/server/internal/usecase/health"
)

func TestHealth_Check(t *testing.T) {
	unreachable := errors.New("connection refused")

	tests := []struct {
		name           string
		redisErr       error
		postgresErr    error
		wantStatusCode int
		wantBody       string
		wantRedis      string
		wantPostgres   string
	}{
		{"both reachable", nil, nil, http.StatusOK, "ok", "ok", "ok"},
		{"redis unreachable", unreachable, nil, http.StatusServiceUnavailable, "degraded", "unreachable", "ok"},
		{"postgres unreachable", nil, unreachable, http.StatusServiceUnavailable, "degraded", "ok", "unreachable"},
		{"both unreachable", unreachable, unreachable, http.StatusServiceUnavailable, "degraded", "unreachable", "unreachable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redis := health.PingerFunc(func(context.Context) error { return tt.redisErr })
			postgres := health.PingerFunc(func(context.Context) error { return tt.postgresErr })
			h := NewHealth(redis, postgres)

			req := httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
			rec := httptest.NewRecorder()
			h.Check(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatusCode)
			}

			var body struct {
				Status   string `json:"status"`
				Redis    string `json:"redis"`
				Postgres string `json:"postgres"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response isn't valid JSON: %v (%s)", err, rec.Body.String())
			}
			if body.Status != tt.wantBody || body.Redis != tt.wantRedis || body.Postgres != tt.wantPostgres {
				t.Errorf("body = %+v, want {status:%s redis:%s postgres:%s}", body, tt.wantBody, tt.wantRedis, tt.wantPostgres)
			}
		})
	}
}
