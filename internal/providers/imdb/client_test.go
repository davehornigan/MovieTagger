package imdb

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

func TestClient_SearchMovie_MapsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("apikey") != "k" {
			t.Fatalf("missing apikey")
		}
		if r.URL.Query().Get("type") != "movie" {
			t.Fatalf("expected movie search type")
		}
		_, _ = w.Write([]byte(`{"Search":[{"Title":"Le Fabuleux Destin d'Amelie Poulain","Year":"2001","imdbID":"tt0211915","Type":"movie"}],"Response":"True"}`))
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

	got, err := c.SearchMovie(context.Background(), model.ProviderSearchCandidate{QueryTitle: "Amelie"})
	if err != nil {
		t.Fatalf("SearchMovie error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Provider != model.ProviderIMDb || got[0].Kind != model.MediaKindMovie {
		t.Fatalf("unexpected provider/kind: %+v", got[0])
	}
	if got[0].Title != "Le Fabuleux Destin d'Amelie Poulain" {
		t.Fatalf("unexpected title: %q", got[0].Title)
	}
	if got[0].Year != 2001 {
		t.Fatalf("unexpected year: %d", got[0].Year)
	}
	if got[0].IDs.IMDbID != "tt0211915" {
		t.Fatalf("unexpected imdb id: %q", got[0].IDs.IMDbID)
	}
}

func TestClient_LookupEpisode_ReturnsEpisodeLevelID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("Season") != "1" || r.URL.Query().Get("Episode") != "2" {
			t.Fatalf("unexpected season/episode query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"Title":"Pilot","Year":"2004","imdbID":"tt1000002","seriesID":"tt1000001","Response":"True"}`))
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
		model.SelectedMatchResult{ProviderReference: "tt1000001"},
		model.EpisodeInfo{SeasonNumber: 1, EpisodeNumber: 2},
	)
	if err != nil {
		t.Fatalf("LookupEpisode error: %v", err)
	}
	if got.EpisodeIDs.IMDbID != "tt1000002" {
		t.Fatalf("expected episode imdb id, got %q", got.EpisodeIDs.IMDbID)
	}
}

func TestClient_RetryAndFatalOnFailure(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "boom", http.StatusInternalServerError)
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

	_, err := c.SearchSeries(context.Background(), model.ProviderSearchCandidate{QueryTitle: "Show"})
	if err == nil {
		t.Fatalf("expected fatal error after retries")
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
	if logger.retryCount < 2 {
		t.Fatalf("expected retry logs, got %d", logger.retryCount)
	}
	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_LookupEpisode_AllowsMissingEpisodeIDForLaterSkip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Title":"Pilot","Year":"2004","imdbID":"","seriesID":"tt1000001","Response":"True"}`))
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
		model.SelectedMatchResult{ProviderReference: "tt1000001"},
		model.EpisodeInfo{SeasonNumber: 1, EpisodeNumber: 2},
	)
	if err != nil {
		t.Fatalf("LookupEpisode error: %v", err)
	}
	if got.EpisodeIDs.IMDbID != "" || got.EpisodeIDs.TMDbID != "" {
		t.Fatalf("expected empty episode ids, got %+v", got.EpisodeIDs)
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
