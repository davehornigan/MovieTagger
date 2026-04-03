package integration_test

import (
	"path/filepath"
	"testing"

	"github.com/davehornigan/MovieTagger/internal/model"
)

func TestPipeline_ValidSeriesRootWithSeasonAndEpisodes_DryRun(t *testing.T) {
	root := t.TempDir()
	series := filepath.Join(root, "My Show")
	season := filepath.Join(series, "Season 1")
	mustMkdir(t, season)
	ep1 := filepath.Join(season, "My.Show.S01E01.mkv")
	ep2 := filepath.Join(season, "My.Show.S01E02.mkv")
	mustWrite(t, ep1)
	mustWrite(t, ep2)

	fp := buildSeriesProvider("My Show", "tt100", "1399", map[string]model.SelectedMatchResult{
		epKey("tt100", 1, 1): {Provider: model.ProviderIMDb, Kind: model.MediaKindEpisode, OriginalTitle: "My Show", EpisodeIDs: model.ProviderTags{IMDbID: "ttE101", TMDbID: "101"}},
		epKey("tt100", 1, 2): {Provider: model.ProviderIMDb, Kind: model.MediaKindEpisode, OriginalTitle: "My Show", EpisodeIDs: model.ProviderTags{IMDbID: "ttE102", TMDbID: "102"}},
	})

	_, err := runPipeline(t, root, true, fp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	assertExists(t, ep1)
	assertExists(t, ep2)
}

func TestPipeline_EpisodeOutsideSeasonIgnoredAndLogged(t *testing.T) {
	root := t.TempDir()
	showDir := filepath.Join(root, "Show")
	mustMkdir(t, showDir)
	ep := filepath.Join(showDir, "Show.S01E01.mkv")
	mustWrite(t, ep)

	_, logger, err := runPipelineWithLogger(t, root, false, &fakeProvider{kind: model.ProviderIMDb})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if !contains(logger.invalidPaths, ep) {
		t.Fatalf("expected invalid structure log for %q", ep)
	}
}

func TestPipeline_SeasonOutsideSeriesIgnoredAndLogged(t *testing.T) {
	root := t.TempDir()
	season := filepath.Join(root, "Season 1")
	mustMkdir(t, season)
	mustWrite(t, filepath.Join(season, "not-episode-video.mkv"))

	_, logger, err := runPipelineWithLogger(t, root, false, &fakeProvider{kind: model.ProviderIMDb})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if !contains(logger.invalidPaths, season) {
		t.Fatalf("expected invalid structure log for %q", season)
	}
}

func TestPipeline_EpisodeFormattingTwoDigits(t *testing.T) {
	root := t.TempDir()
	series := filepath.Join(root, "Show (2011) [imdbid-ttshow]")
	season := filepath.Join(series, "Season 1")
	mustMkdir(t, season)
	ep := filepath.Join(season, "Show.S01E2.mkv")
	mustWrite(t, ep)

	fp := buildSeriesProvider("Show", "ttshow", "", map[string]model.SelectedMatchResult{
		epKey("ttshow", 1, 2): {Provider: model.ProviderIMDb, Kind: model.MediaKindEpisode, OriginalTitle: "Show", EpisodeIDs: model.ProviderTags{IMDbID: "ttep2", TMDbID: "2"}},
	})

	_, err := runPipeline(t, root, false, fp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	assertExists(t, filepath.Join(season, "Show S01E02 [tmdbid-2] [imdbid-ttep2].mkv"))
}

func TestPipeline_EpisodeFormattingThreeDigits(t *testing.T) {
	root := t.TempDir()
	series := filepath.Join(root, "Show (2011) [imdbid-ttshow]")
	season := filepath.Join(series, "Season 1")
	mustMkdir(t, season)
	ep2 := filepath.Join(season, "Show.S01E2.mkv")
	ep100 := filepath.Join(season, "Show.S01E100.mkv")
	mustWrite(t, ep2)
	mustWrite(t, ep100)

	fp := buildSeriesProvider("Show", "ttshow", "", map[string]model.SelectedMatchResult{
		epKey("ttshow", 1, 2):   {Provider: model.ProviderIMDb, Kind: model.MediaKindEpisode, OriginalTitle: "Show", EpisodeIDs: model.ProviderTags{IMDbID: "ttep2", TMDbID: "2"}},
		epKey("ttshow", 1, 100): {Provider: model.ProviderIMDb, Kind: model.MediaKindEpisode, OriginalTitle: "Show", EpisodeIDs: model.ProviderTags{IMDbID: "ttep100", TMDbID: "100"}},
	})

	_, err := runPipeline(t, root, false, fp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	assertExists(t, filepath.Join(season, "Show S01E002 [tmdbid-2] [imdbid-ttep2].mkv"))
	assertExists(t, filepath.Join(season, "Show S01E100 [tmdbid-100] [imdbid-ttep100].mkv"))
}

func TestPipeline_NoEpisodeLevelIDsEpisodeSkipped(t *testing.T) {
	root := t.TempDir()
	series := filepath.Join(root, "Show (2011) [imdbid-ttshow]")
	season := filepath.Join(series, "Season 1")
	mustMkdir(t, season)
	ep := filepath.Join(season, "Show.S01E02.mkv")
	mustWrite(t, ep)

	fp := buildSeriesProvider("Show", "ttshow", "", map[string]model.SelectedMatchResult{
		epKey("ttshow", 1, 2): {Provider: model.ProviderIMDb, Kind: model.MediaKindEpisode, OriginalTitle: "Show"},
	})

	_, err := runPipeline(t, root, false, fp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	assertExists(t, ep)
}

func TestPipeline_SkipSeriesEpisodesLeavesEpisodesUntouched(t *testing.T) {
	root := t.TempDir()
	series := filepath.Join(root, "Show (2011) [imdbid-ttshow]")
	season := filepath.Join(series, "Season 1")
	mustMkdir(t, season)
	ep1 := filepath.Join(season, "Show.S01E01.mkv")
	ep2 := filepath.Join(season, "Show.S01E02.mkv")
	mustWrite(t, ep1)
	mustWrite(t, ep2)

	fp := buildSeriesProvider("Show", "ttshow", "", map[string]model.SelectedMatchResult{
		epKey("ttshow", 1, 1): {Provider: model.ProviderIMDb, Kind: model.MediaKindEpisode, OriginalTitle: "Show", EpisodeIDs: model.ProviderTags{IMDbID: "ttE101", TMDbID: "101"}},
		epKey("ttshow", 1, 2): {Provider: model.ProviderIMDb, Kind: model.MediaKindEpisode, OriginalTitle: "Show", EpisodeIDs: model.ProviderTags{IMDbID: "ttE102", TMDbID: "102"}},
	})

	_, _, err := runPipelineWithLoggerOpts(t, root, false, true, fp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	assertExists(t, ep1)
	assertExists(t, ep2)
	assertNotExists(t, filepath.Join(season, "Show S01E01 [tmdbid-101] [imdbid-ttE101].mkv"))
	assertNotExists(t, filepath.Join(season, "Show S01E02 [tmdbid-102] [imdbid-ttE102].mkv"))
}
