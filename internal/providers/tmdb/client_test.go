package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/davehornigan/MovieTagger/internal/model"
)

func TestClient_SearchMovie_MapsOriginalTitleAndYear(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/search/movie" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":550,"title":"Fight Club","original_title":"Fight Club","release_date":"1999-10-15"}]}`))
	}))
	defer srv.Close()

	c := NewClient(Options{
		APIKey:      "k",
		BaseURL:     srv.URL,
		RetryCount:  3,
		BaseBackoff: time.Millisecond,
		Sleep:       func(time.Duration) {},
	})

	got, err := c.SearchMovie(context.Background(), model.ProviderSearchCandidate{QueryTitle: "Fight Club"})
	if err != nil {
		t.Fatalf("SearchMovie error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one result, got %d", len(got))
	}
	if got[0].Title != "Fight Club" || got[0].Year != 1999 {
		t.Fatalf("unexpected mapped result: %+v", got[0])
	}
	if got[0].IDs.TMDbID != "550" {
		t.Fatalf("unexpected tmdb id: %q", got[0].IDs.TMDbID)
	}
}

func TestClient_LookupEpisode_ReturnsEpisodeLevelIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/3/tv/1399/season/1/episode/1") {
			t.Fatalf("unexpected episode lookup path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":63056,"name":"Winter Is Coming","air_date":"2011-04-17","external_ids":{"imdb_id":"tt1480055"}}`))
	}))
	defer srv.Close()

	c := NewClient(Options{
		APIKey:      "k",
		BaseURL:     srv.URL,
		RetryCount:  3,
		BaseBackoff: time.Millisecond,
		Sleep:       func(time.Duration) {},
	})

	got, err := c.LookupEpisode(
		context.Background(),
		model.SelectedMatchResult{ProviderReference: "1399"},
		model.EpisodeInfo{SeasonNumber: 1, EpisodeNumber: 1},
	)
	if err != nil {
		t.Fatalf("LookupEpisode error: %v", err)
	}
	if got.EpisodeIDs.TMDbID != "63056" {
		t.Fatalf("expected episode tmdb id, got %q", got.EpisodeIDs.TMDbID)
	}
	if got.EpisodeIDs.IMDbID != "tt1480055" {
		t.Fatalf("expected episode imdb id, got %q", got.EpisodeIDs.IMDbID)
	}
}

func TestClient_LookupEpisode_AllowsMissingEpisodeIDsForLaterSkip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":0,"name":"Unknown Episode","air_date":"2011-04-17","external_ids":{"imdb_id":""}}`))
	}))
	defer srv.Close()

	c := NewClient(Options{
		APIKey:      "k",
		BaseURL:     srv.URL,
		RetryCount:  3,
		BaseBackoff: time.Millisecond,
		Sleep:       func(time.Duration) {},
	})

	got, err := c.LookupEpisode(
		context.Background(),
		model.SelectedMatchResult{ProviderReference: "1399"},
		model.EpisodeInfo{SeasonNumber: 1, EpisodeNumber: 1},
	)
	if err != nil {
		t.Fatalf("LookupEpisode error: %v", err)
	}
	if got.EpisodeIDs.IMDbID != "" || got.EpisodeIDs.TMDbID != "" {
		t.Fatalf("expected empty episode ids, got %+v", got.EpisodeIDs)
	}
}

func TestClient_ResolveByKnownIDs_UsesTMDbIDAndReturnsIMDbID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/tv/1399" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("append_to_response") != "external_ids" {
			t.Fatalf("expected external ids append")
		}
		_, _ = w.Write([]byte(`{"id":1399,"name":"Game of Thrones","original_name":"Game of Thrones","first_air_date":"2011-04-17","external_ids":{"imdb_id":"tt0944947"}}`))
	}))
	defer srv.Close()

	c := NewClient(Options{
		APIKey:      "k",
		BaseURL:     srv.URL,
		RetryCount:  3,
		BaseBackoff: time.Millisecond,
		Sleep:       func(time.Duration) {},
	})

	got, err := c.ResolveByKnownIDs(context.Background(), model.ProviderSearchCandidate{
		Kind:     model.MediaKindSeries,
		KnownIDs: model.ProviderTags{TMDbID: "1399"},
	})
	if err != nil {
		t.Fatalf("ResolveByKnownIDs error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].IDs.TMDbID != "1399" || got[0].IDs.IMDbID != "tt0944947" {
		t.Fatalf("unexpected ids: %+v", got[0].IDs)
	}
}

func TestClient_ResolveByKnownIDs_UsesIMDbIDFindEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/find/tt0944947" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("external_source") != "imdb_id" {
			t.Fatalf("expected external_source=imdb_id")
		}
		_, _ = w.Write([]byte(`{"movie_results":[],"tv_results":[{"id":1399,"name":"Game of Thrones","original_name":"Game of Thrones","first_air_date":"2011-04-17"}]}`))
	}))
	defer srv.Close()

	c := NewClient(Options{
		APIKey:      "k",
		BaseURL:     srv.URL,
		RetryCount:  3,
		BaseBackoff: time.Millisecond,
		Sleep:       func(time.Duration) {},
	})

	got, err := c.ResolveByKnownIDs(context.Background(), model.ProviderSearchCandidate{
		Kind:     model.MediaKindSeries,
		KnownIDs: model.ProviderTags{IMDbID: "tt0944947"},
	})
	if err != nil {
		t.Fatalf("ResolveByKnownIDs error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].IDs.IMDbID != "tt0944947" || got[0].IDs.TMDbID != "1399" {
		t.Fatalf("unexpected ids: %+v", got[0].IDs)
	}
}

func TestClient_RetryAndFatalOnFailure(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "fail", http.StatusBadGateway)
	}))
	defer srv.Close()

	logger := &testLogger{}
	c := NewClient(Options{
		APIKey:      "k",
		BaseURL:     srv.URL,
		Logger:      logger,
		RetryCount:  3,
		BaseBackoff: time.Millisecond,
		Sleep:       func(time.Duration) {},
	})

	_, err := c.SearchSeries(context.Background(), model.ProviderSearchCandidate{QueryTitle: "Anything"})
	if err == nil {
		t.Fatalf("expected fatal error after retries")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
	if logger.retryCount < 2 {
		t.Fatalf("expected retries to be logged")
	}
}

func TestClient_LookupEpisode_NotFoundIsNotFatalAndNoRetry(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.NotFound(w, r)
	}))
	defer srv.Close()

	logger := &testLogger{}
	c := NewClient(Options{
		APIKey:      "k",
		BaseURL:     srv.URL,
		Logger:      logger,
		RetryCount:  3,
		BaseBackoff: time.Millisecond,
		Sleep:       func(time.Duration) {},
	})

	got, err := c.LookupEpisode(
		context.Background(),
		model.SelectedMatchResult{ProviderReference: "1399"},
		model.EpisodeInfo{SeasonNumber: 1, EpisodeNumber: 1},
	)
	if err != nil {
		t.Fatalf("expected no error for not found episode, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected no retries for not found episode, calls=%d", calls)
	}
	if logger.retryCount != 0 {
		t.Fatalf("expected no retry logs, got %d", logger.retryCount)
	}
	if got != (model.SelectedMatchResult{}) {
		t.Fatalf("expected empty result for not found episode, got %+v", got)
	}
}

type testLogger struct {
	mu         sync.Mutex
	retryCount int
}

func (l *testLogger) Infof(string, ...any)                       {}
func (l *testLogger) Warnf(string, ...any)                       {}
func (l *testLogger) Errorf(string, ...any)                      {}
func (l *testLogger) LogScanStart(string)                        {}
func (l *testLogger) LogScanEnd(string, time.Duration, error)    {}
func (l *testLogger) LogProviderCall(model.ProviderKind, string) {}
func (l *testLogger) LogProviderRetry(model.ProviderKind, string, int, error) {
	l.mu.Lock()
	l.retryCount++
	l.mu.Unlock()
}
func (l *testLogger) LogMatch(string, model.SelectedMatchResult) {}
func (l *testLogger) LogRenamePlan(model.RenamePlan)             {}
func (l *testLogger) LogSkip(string, string)                     {}
func (l *testLogger) LogCollision(string, []string)              {}
func (l *testLogger) LogInvalidSeriesStructure(string, string)   {}
func (l *testLogger) Close() error                               { return nil }
