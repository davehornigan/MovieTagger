package tmdb

import (
	"github.com/davehornigan/MovieTagger/internal/logging"
	"github.com/davehornigan/MovieTagger/internal/model"
	"github.com/davehornigan/MovieTagger/internal/providers"
)

type Provider struct {
	movieSeries providers.MovieSeriesLookupClient
	episode     providers.EpisodeLookupClient
}

func New(movieSeries providers.MovieSeriesLookupClient, episode providers.EpisodeLookupClient) *Provider {
	return &Provider{movieSeries: movieSeries, episode: episode}
}

func (p *Provider) Kind() model.ProviderKind {
	return model.ProviderTMDb
}

func (p *Provider) MovieSeriesClient() providers.MovieSeriesLookupClient {
	return p.movieSeries
}

func (p *Provider) EpisodeClient() providers.EpisodeLookupClient {
	return p.episode
}

func NewFromAPIKey(apiKey string, logger logging.Logger) *Provider {
	client := NewClient(Options{
		APIKey: apiKey,
		Logger: logger,
	})
	return New(client, client)
}
