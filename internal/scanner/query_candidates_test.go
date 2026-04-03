package scanner

import (
	"context"
	"testing"

	"github.com/davehornigan/MovieTagger/internal/model"
	"github.com/davehornigan/MovieTagger/internal/providers"
)

func TestQueryCandidates_UsesKnownIDResolutionBeforeSearch(t *testing.T) {
	client := &idAwareMovieSeriesClient{
		resolveMovie: []model.SelectedMatchResult{
			{
				Provider: model.ProviderTMDb,
				Kind:     model.MediaKindMovie,
				Title:    "Valera",
				IDs: model.ProviderTags{
					IMDbID: "tt1234567",
					TMDbID: "98765",
				},
			},
		},
	}
	provider := &stubProvider{kind: model.ProviderTMDb, movieSeries: client, episode: &stubEpisodeClient{}}
	s := New(Options{
		NoInteractive: true,
		Providers:     []providers.MetadataProvider{provider},
	})

	item := model.ScanResultItem{
		Path: "/media/Valera (2003) [imdbid-tt1234567].mp4",
		Kind: model.MediaKindMovie,
		Parsed: model.ParsedFilenameInfo{
			TitleHint: "Valera",
			YearHint:  2003,
			ExistingFileIDs: model.ProviderTags{
				IMDbID: "tt1234567",
			},
		},
	}

	got, err := s.queryCandidates(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("queryCandidates failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}
	if client.resolveCalls != 1 {
		t.Fatalf("expected resolve calls=1, got %d", client.resolveCalls)
	}
	if client.searchMovieCalls != 0 {
		t.Fatalf("expected search movie calls=0 when resolve succeeds, got %d", client.searchMovieCalls)
	}
}

func TestQueryCandidates_FallsBackToSearchWhenKnownIDResolutionEmpty(t *testing.T) {
	client := &idAwareMovieSeriesClient{
		resolveMovie: nil,
		searchMovie: []model.SelectedMatchResult{
			{
				Provider: model.ProviderTMDb,
				Kind:     model.MediaKindMovie,
				Title:    "Valera",
				IDs:      model.ProviderTags{TMDbID: "98765"},
			},
		},
	}
	provider := &stubProvider{kind: model.ProviderTMDb, movieSeries: client, episode: &stubEpisodeClient{}}
	s := New(Options{
		NoInteractive: true,
		Providers:     []providers.MetadataProvider{provider},
	})

	item := model.ScanResultItem{
		Path: "/media/Valera (2003) [imdbid-tt1234567].mp4",
		Kind: model.MediaKindMovie,
		Parsed: model.ParsedFilenameInfo{
			TitleHint: "Valera",
			YearHint:  2003,
			ExistingFileIDs: model.ProviderTags{
				IMDbID: "tt1234567",
			},
		},
	}

	got, err := s.queryCandidates(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("queryCandidates failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}
	if client.resolveCalls != 1 {
		t.Fatalf("expected resolve calls=1, got %d", client.resolveCalls)
	}
	if client.searchMovieCalls != 1 {
		t.Fatalf("expected search movie fallback call=1, got %d", client.searchMovieCalls)
	}
}

type stubProvider struct {
	kind        model.ProviderKind
	movieSeries providers.MovieSeriesLookupClient
	episode     providers.EpisodeLookupClient
}

func (s *stubProvider) Kind() model.ProviderKind                             { return s.kind }
func (s *stubProvider) MovieSeriesClient() providers.MovieSeriesLookupClient { return s.movieSeries }
func (s *stubProvider) EpisodeClient() providers.EpisodeLookupClient         { return s.episode }

type stubEpisodeClient struct{}

func (s *stubEpisodeClient) LookupEpisode(context.Context, model.SelectedMatchResult, model.EpisodeInfo) (model.SelectedMatchResult, error) {
	return model.SelectedMatchResult{}, nil
}

type idAwareMovieSeriesClient struct {
	resolveCalls     int
	searchMovieCalls int

	resolveMovie []model.SelectedMatchResult
	searchMovie  []model.SelectedMatchResult
}

func (c *idAwareMovieSeriesClient) ResolveByKnownIDs(_ context.Context, candidate model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error) {
	c.resolveCalls++
	if !candidate.KnownIDs.HasAny() {
		return nil, nil
	}
	return c.resolveMovie, nil
}

func (c *idAwareMovieSeriesClient) SearchMovie(_ context.Context, _ model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error) {
	c.searchMovieCalls++
	return c.searchMovie, nil
}

func (c *idAwareMovieSeriesClient) SearchSeries(_ context.Context, _ model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error) {
	return nil, nil
}
