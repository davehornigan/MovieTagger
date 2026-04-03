package providers

import (
	"context"

	"github.com/davehornigan/MovieTagger/internal/model"
)

// MovieSeriesLookupClient resolves movie or series metadata candidates.
type MovieSeriesLookupClient interface {
	SearchMovie(ctx context.Context, candidate model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error)
	SearchSeries(ctx context.Context, candidate model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error)
}

// IDAwareMovieSeriesLookupClient can resolve exact movie/series candidates using known IDs.
// Implementations may return an empty slice when ID-based resolution is not possible.
type IDAwareMovieSeriesLookupClient interface {
	ResolveByKnownIDs(ctx context.Context, candidate model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error)
}

// EpisodeLookupClient resolves episode-level metadata.
type EpisodeLookupClient interface {
	LookupEpisode(ctx context.Context, series model.SelectedMatchResult, episode model.EpisodeInfo) (model.SelectedMatchResult, error)
}

// MetadataProvider bundles provider identity and lookup capabilities.
type MetadataProvider interface {
	Kind() model.ProviderKind
	MovieSeriesClient() MovieSeriesLookupClient
	EpisodeClient() EpisodeLookupClient
}
