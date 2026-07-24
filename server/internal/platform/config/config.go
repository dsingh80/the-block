// Package config is the small env-driven settings struct shared by every cmd/*
// binary (guidelines/06-backend-architecture.md, "Code organization"). Grows as
// later commits need more (a Redis URL, session TTLs, ...), not pre-declared now.
package config

import "os"

type Config struct {
	DatabaseURL      string
	VehiclesDataPath string
	RedisAddr        string
}

func Load() Config {
	return Config{
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/theblock?sslmode=disable"),
		VehiclesDataPath: getEnv("VEHICLES_DATA_PATH", "../data/vehicles.json"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
