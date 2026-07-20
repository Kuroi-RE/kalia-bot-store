package config

import (
	"testing"
	"time"
)

func TestGetEnvDuration(t *testing.T) {
	t.Setenv("SOME_TTL", "30s")
	if got := getEnvDuration("SOME_TTL", time.Minute); got != 30*time.Second {
		t.Fatalf("expected 30s, got %v", got)
	}

	t.Setenv("SECS_TTL", "900")
	if got := getEnvDuration("SECS_TTL", time.Minute); got != 15*time.Minute {
		t.Fatalf("expected 15m, got %v", got)
	}

	if got := getEnvDuration("MISSING_TTL", 5*time.Minute); got != 5*time.Minute {
		t.Fatalf("expected fallback 5m, got %v", got)
	}
}

func TestValidate(t *testing.T) {
	c := &Config{}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for empty config")
	}

	c = &Config{DatabaseURL: "x", JWTSecret: "y", BotServiceToken: "z"}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}
