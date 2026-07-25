package config

import "testing"

func TestLoad_UsesDefaultsWhenEnvUnset(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "VEHICLES_DATA_PATH", "REDIS_ADDR", "PORT", "PUBLIC_ORIGIN"} {
		t.Setenv(key, "")
	}

	cfg := Load()
	if cfg.DatabaseURL == "" || cfg.VehiclesDataPath == "" || cfg.RedisAddr == "" || cfg.Port == "" || cfg.PublicOrigin == "" {
		t.Errorf("Load() with no env set = %+v, want every field to have a non-empty default", cfg)
	}
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://custom/db")
	t.Setenv("VEHICLES_DATA_PATH", "/custom/path.json")
	t.Setenv("REDIS_ADDR", "custom:6380")
	t.Setenv("PORT", "9090")
	t.Setenv("PUBLIC_ORIGIN", "https://example.com")

	got := Load()
	want := Config{
		DatabaseURL:      "postgres://custom/db",
		VehiclesDataPath: "/custom/path.json",
		RedisAddr:        "custom:6380",
		Port:             "9090",
		PublicOrigin:     "https://example.com",
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}
