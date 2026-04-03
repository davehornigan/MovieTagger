package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/davehornigan/MovieTagger/internal/model"
)

// ProviderConfig contains settings for one metadata provider.
type ProviderConfig struct {
	APIKey string `yaml:"api_key"`
}

// Config contains runtime settings loaded from YAML.
type Config struct {
	Path string         `yaml:"-"`
	IMDb ProviderConfig `yaml:"imdb"`
	TMDb ProviderConfig `yaml:"tmdb"`
}

// ProviderStatus describes if a provider can be used for this run.
type ProviderStatus struct {
	Kind      model.ProviderKind
	Disabled  bool
	HasAPIKey bool
	Available bool
	Reason    string
}

// ProviderAvailability contains resolved provider statuses.
type ProviderAvailability struct {
	IMDb ProviderStatus
	TMDb ProviderStatus
}

func (p ProviderAvailability) AvailableKinds() []model.ProviderKind {
	kinds := make([]model.ProviderKind, 0, 2)
	if p.IMDb.Available {
		kinds = append(kinds, model.ProviderIMDb)
	}
	if p.TMDb.Available {
		kinds = append(kinds, model.ProviderTMDb)
	}
	return kinds
}

func (p ProviderAvailability) HasAnyAvailable() bool {
	return p.IMDb.Available || p.TMDb.Available
}

func (p ProviderAvailability) UnavailableReasons() []string {
	reasons := make([]string, 0, 2)
	if !p.IMDb.Available {
		reasons = append(reasons, fmt.Sprintf("imdb unavailable: %s", p.IMDb.Reason))
	}
	if !p.TMDb.Available {
		reasons = append(reasons, fmt.Sprintf("tmdb unavailable: %s", p.TMDb.Reason))
	}
	return reasons
}

func (c Config) EnabledProviders() []model.ProviderKind {
	providers := make([]model.ProviderKind, 0, 2)
	if strings.TrimSpace(c.IMDb.APIKey) != "" {
		providers = append(providers, model.ProviderIMDb)
	}
	if strings.TrimSpace(c.TMDb.APIKey) != "" {
		providers = append(providers, model.ProviderTMDb)
	}
	return providers
}

func Load(path string) (Config, error) {
	cfg := Config{Path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}

	if len(data) == 0 {
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode yaml: %w", err)
	}

	cfg.Path = path
	cfg.IMDb.APIKey = strings.TrimSpace(cfg.IMDb.APIKey)
	cfg.TMDb.APIKey = strings.TrimSpace(cfg.TMDb.APIKey)
	return cfg, nil
}

func ResolveProviderAvailability(cfg Config, disableIMDb, disableTMDB bool) ProviderAvailability {
	statusIMDb := ProviderStatus{
		Kind:      model.ProviderIMDb,
		Disabled:  disableIMDb,
		HasAPIKey: cfg.IMDb.APIKey != "",
	}
	statusTMDb := ProviderStatus{
		Kind:      model.ProviderTMDb,
		Disabled:  disableTMDB,
		HasAPIKey: cfg.TMDb.APIKey != "",
	}

	if statusIMDb.Disabled {
		statusIMDb.Reason = "disabled by flag"
	} else if !statusIMDb.HasAPIKey {
		statusIMDb.Reason = "missing api_key in config"
	} else {
		statusIMDb.Available = true
	}

	if statusTMDb.Disabled {
		statusTMDb.Reason = "disabled by flag"
	} else if !statusTMDb.HasAPIKey {
		statusTMDb.Reason = "missing api_key in config"
	} else {
		statusTMDb.Available = true
	}

	return ProviderAvailability{
		IMDb: statusIMDb,
		TMDb: statusTMDb,
	}
}
