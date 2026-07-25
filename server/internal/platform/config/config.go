// Package config is the small env-driven settings struct shared by every cmd/*
// binary (guidelines/06-backend-architecture.md, "Code organization"). Grows as
// later commits need more (a Redis URL, session TTLs, ...), not pre-declared now.
package config

import "os"

type Config struct {
	DatabaseURL      string
	VehiclesDataPath string
	RedisAddr        string
	// Port is the plain-HTTP port cmd/api listens on internally -- Caddy is the
	// only thing that terminates TLS or is reachable from outside the compose
	// network (guidelines/06-backend-architecture.md, "Deployment & HTTPS").
	Port string
	// PublicOrigin is the single origin the whole app is served under (Caddy
	// proxies both the client and /v1/* behind it), used as the WS handshake's
	// Origin allow-list (guidelines/06-backend-architecture.md, "WebSocket protocol").
	PublicOrigin string
}

func Load() Config {
	return Config{
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/theblock?sslmode=disable"),
		VehiclesDataPath: getEnv("VEHICLES_DATA_PATH", "../data/vehicles.json"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		Port:             getEnv("PORT", "8080"),
		PublicOrigin:     getEnv("PUBLIC_ORIGIN", "https://localhost"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
