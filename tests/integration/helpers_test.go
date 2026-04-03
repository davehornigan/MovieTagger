package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/davehornigan/MovieTagger/internal/model"
	"github.com/davehornigan/MovieTagger/internal/providers"
	"github.com/davehornigan/MovieTagger/internal/scanner"
)

func runPipeline(t *testing.T, root string, dryRun bool, fake providers.MetadataProvider) (*captureLogger, error) {
	t.Helper()
	_, logger, err := runPipelineWithLogger(t, root, dryRun, fake)
	return logger, err
}

func runPipelineWithLogger(t *testing.T, root string, dryRun bool, fake providers.MetadataProvider) (*scanner.Scanner, *captureLogger, error) {
	t.Helper()
	logger := &captureLogger{}
	s := scanner.New(scanner.Options{
		NoInteractive: true,
		DryRun:        dryRun,
		Logger:        logger,
		Providers:     []providers.MetadataProvider{fake},
	})
	err := s.Scan(context.Background(), root)
	return s, logger, err
}

type fakeProvider struct {
	kind    model.ProviderKind
	movies  map[string][]model.SelectedMatchResult
	series  map[string][]model.SelectedMatchResult
	episode map[string]model.SelectedMatchResult
}

func (f *fakeProvider) Kind() model.ProviderKind                             { return f.kind }
func (f *fakeProvider) MovieSeriesClient() providers.MovieSeriesLookupClient { return f }
func (f *fakeProvider) EpisodeClient() providers.EpisodeLookupClient         { return f }

func (f *fakeProvider) SearchMovie(_ context.Context, c model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error) {
	if f.movies == nil {
		return nil, nil
	}
	res := f.movies[norm(c.QueryTitle)]
	return normalizeCandidates(res, f.kind, model.MediaKindMovie), nil
}

func (f *fakeProvider) SearchSeries(_ context.Context, c model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error) {
	if f.series == nil {
		return nil, nil
	}
	res := f.series[norm(c.QueryTitle)]
	return normalizeCandidates(res, f.kind, model.MediaKindSeries), nil
}

func (f *fakeProvider) LookupEpisode(_ context.Context, series model.SelectedMatchResult, episode model.EpisodeInfo) (model.SelectedMatchResult, error) {
	if f.episode == nil {
		return model.SelectedMatchResult{Provider: f.kind, Kind: model.MediaKindEpisode}, nil
	}
	id := series.ProviderReference
	if id == "" {
		id = series.IDs.IMDbID
	}
	if id == "" {
		id = series.IDs.TMDbID
	}
	k := epKey(id, episode.SeasonNumber, episode.EpisodeNumber)
	res, ok := f.episode[k]
	if !ok {
		return model.SelectedMatchResult{Provider: f.kind, Kind: model.MediaKindEpisode}, nil
	}
	if res.Provider == "" {
		res.Provider = f.kind
	}
	if res.Kind == "" {
		res.Kind = model.MediaKindEpisode
	}
	if res.Episode == nil {
		res.Episode = &model.EpisodeInfo{SeasonNumber: episode.SeasonNumber, EpisodeNumber: episode.EpisodeNumber}
	}
	return res, nil
}

func buildSeriesProvider(title string, imdbID string, tmdbID string, episodes map[string]model.SelectedMatchResult) *fakeProvider {
	return &fakeProvider{
		kind: model.ProviderIMDb,
		series: map[string][]model.SelectedMatchResult{
			norm(title): {{
				Provider:          model.ProviderIMDb,
				Kind:              model.MediaKindSeries,
				Title:             title,
				OriginalTitle:     title,
				Year:              2011,
				IDs:               model.ProviderTags{IMDbID: imdbID, TMDbID: tmdbID},
				ProviderReference: imdbID,
			}},
		},
		episode: episodes,
	}
}

func normalizeCandidates(in []model.SelectedMatchResult, provider model.ProviderKind, kind model.MediaKind) []model.SelectedMatchResult {
	out := make([]model.SelectedMatchResult, 0, len(in))
	for _, r := range in {
		if r.Provider == "" {
			r.Provider = provider
		}
		if r.Kind == "" {
			r.Kind = kind
		}
		out = append(out, r)
	}
	return out
}

func norm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func epKey(seriesID string, season int, episode int) string {
	return fmt.Sprintf("%s:%d:%d", strings.TrimSpace(seriesID), season, episode)
}

type captureLogger struct {
	mu           sync.Mutex
	invalidPaths []string
}

func (l *captureLogger) Infof(string, ...any)                                    {}
func (l *captureLogger) Warnf(string, ...any)                                    {}
func (l *captureLogger) Errorf(string, ...any)                                   {}
func (l *captureLogger) LogScanStart(string)                                     {}
func (l *captureLogger) LogScanEnd(string, time.Duration, error)                 {}
func (l *captureLogger) LogProviderCall(model.ProviderKind, string)              {}
func (l *captureLogger) LogProviderRetry(model.ProviderKind, string, int, error) {}
func (l *captureLogger) LogMatch(string, model.SelectedMatchResult)              {}
func (l *captureLogger) LogRenamePlan(model.RenamePlan)                          {}
func (l *captureLogger) LogSkip(string, string)                                  {}
func (l *captureLogger) LogCollision(string, []string)                           {}
func (l *captureLogger) LogInvalidSeriesStructure(path string, _ string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.invalidPaths = append(l.invalidPaths, path)
}
func (l *captureLogger) Close() error { return nil }

func contains(items []string, v string) bool {
	for _, it := range items {
		if it == v {
			return true
		}
	}
	return false
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent for %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %q to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q not to exist, got err=%v", path, err)
	}
}
