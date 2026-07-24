package dto

// HealthCheck is the wire shape of GET /healthcheck. redis/postgres are each
// "ok" or "unreachable" independently, so an operator can tell which
// dependency is the problem without cross-referencing logs first.
type HealthCheck struct {
	Status   string `json:"status"` // "ok" | "degraded"
	Redis    string `json:"redis"`
	Postgres string `json:"postgres"`
}
