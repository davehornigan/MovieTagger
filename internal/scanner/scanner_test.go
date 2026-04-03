package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/davehornigan/MovieTagger/internal/model"
)

func TestScanResult_ValidSeriesRootAndSeasonRecognition(t *testing.T) {
	root := t.TempDir()
	seriesRoot := filepath.Join(root, "My Show")
	season1 := filepath.Join(seriesRoot, "Season 1")
	specials := filepath.Join(seriesRoot, "Specials")

	mustMkdirAll(t, season1)
	mustMkdirAll(t, specials)
	mustWriteFile(t, filepath.Join(season1, "My.Show.S01E01.mkv"))
	mustWriteFile(t, filepath.Join(specials, "My.Show.S00E01.mkv"))

	s := New(Options{})
	res, err := s.ScanResult(context.Background(), root)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if !hasItem(res.Items, seriesRoot, model.MediaKindSeries, true) {
		t.Fatalf("expected valid series root item for %q", seriesRoot)
	}
	if !hasSeasonItem(res.Items, season1) {
		t.Fatalf("expected season folder item for %q", season1)
	}
	if !hasSeasonItem(res.Items, specials) {
		t.Fatalf("expected season folder item for %q", specials)
	}
	if !hasItem(res.Items, filepath.Join(season1, "My.Show.S01E01.mkv"), model.MediaKindEpisode, false) {
		t.Fatalf("expected valid episode item in season")
	}
}

func TestScanResult_InvalidSeasonOutsideSeries(t *testing.T) {
	root := t.TempDir()
	seasonOnly := filepath.Join(root, "Season 1")
	mustMkdirAll(t, seasonOnly)
	// No episode inside this season folder, so parent cannot become a valid series root.
	mustWriteFile(t, filepath.Join(seasonOnly, "not-an-episode-video.mkv"))

	s := New(Options{})
	res, err := s.ScanResult(context.Background(), root)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if !hasInvalidFinding(res.InvalidTVFindings, model.InvalidSeasonOutsideSeries, seasonOnly) {
		t.Fatalf("expected invalid season outside series finding for %q", seasonOnly)
	}
	if hasItem(res.Items, seasonOnly, model.MediaKindSeries, true) {
		t.Fatalf("did not expect series root item for invalid season parent")
	}
}

func TestScanResult_InvalidEpisodeOutsideSeason(t *testing.T) {
	root := t.TempDir()
	showDir := filepath.Join(root, "Show")
	mustMkdirAll(t, showDir)
	ep := filepath.Join(showDir, "Show.S01E01.mkv")
	mustWriteFile(t, ep)

	s := New(Options{})
	res, err := s.ScanResult(context.Background(), root)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if !hasInvalidFinding(res.InvalidTVFindings, model.InvalidEpisodeOutsideSeason, ep) {
		t.Fatalf("expected invalid episode outside season finding for %q", ep)
	}
	if hasItem(res.Items, ep, model.MediaKindEpisode, false) {
		t.Fatalf("did not expect episode item for invalid hierarchy")
	}
}

func TestScanResult_InvalidEpisodeInsideSeasonButSeriesInvalid(t *testing.T) {
	root := t.TempDir()
	// Nested season folders make the immediate parent season invalid as a series root.
	notSeriesRoot := filepath.Join(root, "Season 1")
	seasonDir := filepath.Join(notSeriesRoot, "Season 2")
	mustMkdirAll(t, seasonDir)
	ep := filepath.Join(seasonDir, "Episode.S01E03.mkv")
	mustWriteFile(t, ep)

	s := New(Options{})
	res, err := s.ScanResult(context.Background(), root)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if !hasInvalidFinding(res.InvalidTVFindings, model.InvalidSeasonOutsideSeries, seasonDir) {
		t.Fatalf("expected season outside series finding for %q", seasonDir)
	}
	if !hasInvalidFinding(res.InvalidTVFindings, model.InvalidEpisodeInBadSeries, ep) {
		t.Fatalf("expected episode in invalid series finding for %q", ep)
	}
	if hasItem(res.Items, ep, model.MediaKindEpisode, false) {
		t.Fatalf("did not expect valid episode item for %q", ep)
	}
}

func TestScanResult_MixedDirectoriesTraversed(t *testing.T) {
	root := t.TempDir()
	genreDir := filepath.Join(root, "Action")
	notSeriesFolder := filepath.Join(genreDir, "Movies")
	mustMkdirAll(t, notSeriesFolder)
	movie := filepath.Join(notSeriesFolder, "Valera (2003).mp4")
	mustWriteFile(t, movie)

	seriesRoot := filepath.Join(root, "TV", "Some Show")
	season := filepath.Join(seriesRoot, "Season 1")
	mustMkdirAll(t, season)
	mustWriteFile(t, filepath.Join(season, "Some.Show.S01E01.mkv"))

	s := New(Options{})
	res, err := s.ScanResult(context.Background(), root)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// Non-series directories are traversed and movie files are still discovered.
	if !hasItem(res.Items, movie, model.MediaKindMovie, false) {
		t.Fatalf("expected movie item found under non-series directory")
	}
	// Valid series still recognized elsewhere in tree.
	if !hasItem(res.Items, seriesRoot, model.MediaKindSeries, true) {
		t.Fatalf("expected valid series root in mixed tree")
	}
	// Non-series directories themselves are not marked as series roots.
	if hasItem(res.Items, genreDir, model.MediaKindSeries, true) {
		t.Fatalf("did not expect non-series directory to be marked as series root")
	}
}

func hasItem(items []model.ScanResultItem, path string, kind model.MediaKind, isDir bool) bool {
	for _, it := range items {
		if it.Path == path && it.Kind == kind && it.IsDir == isDir {
			return true
		}
	}
	return false
}

func hasSeasonItem(items []model.ScanResultItem, path string) bool {
	for _, it := range items {
		if it.Path == path && it.IsDir && it.Season != nil && it.Season.Valid {
			return true
		}
	}
	return false
}

func hasInvalidFinding(findings []model.InvalidTVFinding, typ model.InvalidTVFindingType, path string) bool {
	for _, f := range findings {
		if f.Type == typ && f.Path == path {
			return true
		}
	}
	return false
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
