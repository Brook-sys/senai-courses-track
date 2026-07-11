package appconfig

import "testing"

func TestFromEnvUsesContainerFriendlyOverrides(t *testing.T) {
	t.Setenv("SENAI_TRACK_ADDR", "0.0.0.0:9000")
	t.Setenv("SENAI_TRACK_DB_PATH", "/data/custom.db")

	cfg := FromEnv()

	if cfg.Addr != "0.0.0.0:9000" {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, "0.0.0.0:9000")
	}
	if cfg.DBPath != "/data/custom.db" {
		t.Fatalf("DBPath = %q, want %q", cfg.DBPath, "/data/custom.db")
	}
}

func TestFromEnvUsesLocalDefaults(t *testing.T) {
	t.Setenv("SENAI_TRACK_ADDR", "")
	t.Setenv("SENAI_TRACK_DB_PATH", "")

	cfg := FromEnv()

	if cfg.Addr != ":8020" {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, ":8020")
	}
	if cfg.DBPath != "courses.db" {
		t.Fatalf("DBPath = %q, want %q", cfg.DBPath, "courses.db")
	}
}
