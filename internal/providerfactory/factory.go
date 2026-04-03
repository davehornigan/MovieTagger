package providerfactory

import (
	"strings"

	"github.com/davehornigan/MovieTagger/internal/logging"
	"github.com/davehornigan/MovieTagger/internal/model"
	"github.com/davehornigan/MovieTagger/internal/providers"
	"github.com/davehornigan/MovieTagger/internal/providers/imdb"
	"github.com/davehornigan/MovieTagger/internal/providers/tmdb"
)

type BuildOptions struct {
	IMDbAPIKey  string
	TMDbAPIKey  string
	DisableIMDb bool
	DisableTMDb bool
	Logger      logging.Logger
}

// Build returns providers in fixed priority order:
// 1) IMDb
// 2) TMDb
func Build(opts BuildOptions) []providers.MetadataProvider {
	out := make([]providers.MetadataProvider, 0, 2)

	if !opts.DisableIMDb && strings.TrimSpace(opts.IMDbAPIKey) != "" {
		out = append(out, imdb.NewFromAPIKey(opts.IMDbAPIKey, opts.Logger))
	}
	if !opts.DisableTMDb && strings.TrimSpace(opts.TMDbAPIKey) != "" {
		out = append(out, tmdb.NewFromAPIKey(opts.TMDbAPIKey, opts.Logger))
	}
	return out
}

func PriorityOrder() []model.ProviderKind {
	return []model.ProviderKind{model.ProviderIMDb, model.ProviderTMDb}
}
