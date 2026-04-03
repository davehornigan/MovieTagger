package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davehornigan/MovieTagger/internal/model"
)

func TestLoad_DefaultPriorityProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.PriorityProvider != model.ProviderTMDb {
		t.Fatalf("expected default priority provider tmdb, got %q", cfg.PriorityProvider)
	}
}

func TestLoad_NormalizePriorityProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := []byte("priority_provider: IMDb\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.PriorityProvider != model.ProviderIMDb {
		t.Fatalf("expected imdb after normalize, got %q", cfg.PriorityProvider)
	}
}

func TestLoad_InvalidPriorityProviderFallsBackToTMDb(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := []byte("priority_provider: bad-value\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.PriorityProvider != model.ProviderTMDb {
		t.Fatalf("expected fallback to tmdb, got %q", cfg.PriorityProvider)
	}
}
