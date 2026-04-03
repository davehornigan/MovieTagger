package integration_test

import (
	"path/filepath"
	"testing"

	"github.com/davehornigan/MovieTagger/internal/model"
)

func TestPipeline_MovieWithSubtitlesAndPoster(t *testing.T) {
	root := t.TempDir()
	movie := filepath.Join(root, "Movie.mp4")
	sub := filepath.Join(root, "Movie.en.srt")
	poster := filepath.Join(root, "Movie-cover.jpg")
	mustWrite(t, movie)
	mustWrite(t, sub)
	mustWrite(t, poster)

	fp := &fakeProvider{kind: model.ProviderIMDb,
		movies: map[string][]model.SelectedMatchResult{
			norm("Movie"): {{
				Provider:          model.ProviderIMDb,
				Kind:              model.MediaKindMovie,
				OriginalTitle:     "The Movie",
				Year:              2000,
				IDs:               model.ProviderTags{IMDbID: "tt1234567", TMDbID: "98765"},
				ProviderReference: "tt1234567",
			}},
		},
	}

	_, err := runPipeline(t, root, false, fp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	assertExists(t, filepath.Join(root, "The Movie (2000) [imdbid-tt1234567] [tmdbid-98765].mp4"))
	assertExists(t, filepath.Join(root, "The Movie (2000) [imdbid-tt1234567] [tmdbid-98765].en.srt"))
	assertExists(t, filepath.Join(root, "The Movie (2000) [imdbid-tt1234567] [tmdbid-98765]-cover.jpg"))
}

func TestPipeline_DirtyReleaseStyleMovieName(t *testing.T) {
	root := t.TempDir()
	name := "L.Amour.et.les.Forets.2023.D.BDRip.1.46Gb.MegaPeer.avi"
	mustWrite(t, filepath.Join(root, name))

	fp := &fakeProvider{kind: model.ProviderIMDb,
		movies: map[string][]model.SelectedMatchResult{
			norm("L Amour et les Forets"): {{
				Provider:          model.ProviderIMDb,
				Kind:              model.MediaKindMovie,
				OriginalTitle:     "L'Amour et les Forets",
				Year:              2023,
				IDs:               model.ProviderTags{IMDbID: "tt777", TMDbID: "777"},
				ProviderReference: "tt777",
			}},
		},
	}

	_, err := runPipeline(t, root, false, fp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	assertExists(t, filepath.Join(root, "L'Amour et les Forets (2023) [imdbid-tt777] [tmdbid-777].avi"))
}

func TestPipeline_PartiallyTaggedMovieEnrichment(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "Valera (2003) [imdbid-tt1234567].mp4")
	mustWrite(t, from)

	fp := &fakeProvider{kind: model.ProviderIMDb,
		movies: map[string][]model.SelectedMatchResult{
			norm("Valera"): {{
				Provider:          model.ProviderIMDb,
				Kind:              model.MediaKindMovie,
				OriginalTitle:     "Valera",
				Year:              2003,
				IDs:               model.ProviderTags{IMDbID: "tt1234567", TMDbID: "98765"},
				ProviderReference: "tt1234567",
			}},
		},
	}

	_, err := runPipeline(t, root, false, fp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	to := filepath.Join(root, "Valera (2003) [imdbid-tt1234567] [tmdbid-98765].mp4")
	assertExists(t, to)
}

func TestPipeline_NonSeriesDirectoryTraversedButNotRenamed(t *testing.T) {
	root := t.TempDir()
	genre := filepath.Join(root, "Action")
	moviesDir := filepath.Join(genre, "Movies")
	mustMkdir(t, moviesDir)
	movie := filepath.Join(moviesDir, "Valera (2003).mp4")
	mustWrite(t, movie)

	fp := &fakeProvider{kind: model.ProviderIMDb,
		movies: map[string][]model.SelectedMatchResult{
			norm("Valera"): {{
				Provider: model.ProviderIMDb, Kind: model.MediaKindMovie, OriginalTitle: "Valera", Year: 2003,
				IDs: model.ProviderTags{IMDbID: "tt11"}, ProviderReference: "tt11",
			}},
		},
	}
	_, err := runPipeline(t, root, false, fp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	assertExists(t, genre)
	assertExists(t, filepath.Join(moviesDir, "Valera (2003) [imdbid-tt11].mp4"))
}

func TestPipeline_DryRun(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "Movie.mp4")
	mustWrite(t, from)

	fp := &fakeProvider{kind: model.ProviderIMDb,
		movies: map[string][]model.SelectedMatchResult{
			norm("Movie"): {{Provider: model.ProviderIMDb, Kind: model.MediaKindMovie, OriginalTitle: "The Movie", Year: 2000, IDs: model.ProviderTags{IMDbID: "tt1"}, ProviderReference: "tt1"}},
		},
	}
	_, err := runPipeline(t, root, true, fp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	assertExists(t, from)
	assertNotExists(t, filepath.Join(root, "The Movie (2000) [imdbid-tt1].mp4"))
}

func TestPipeline_CollisionHandling(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "A.mp4"))
	mustWrite(t, filepath.Join(root, "B.mp4"))

	fp := &fakeProvider{kind: model.ProviderIMDb,
		movies: map[string][]model.SelectedMatchResult{
			norm("A"): {{Provider: model.ProviderIMDb, Kind: model.MediaKindMovie, OriginalTitle: "Same", Year: 2000}},
			norm("B"): {{Provider: model.ProviderIMDb, Kind: model.MediaKindMovie, OriginalTitle: "Same", Year: 2000}},
		},
	}
	_, err := runPipeline(t, root, false, fp)
	if err == nil {
		t.Fatalf("expected collision error")
	}
}

func TestPipeline_UnicodeCyrillicFilenames(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "Валера.2003.BDRip.avi")
	mustWrite(t, from)

	fp := &fakeProvider{kind: model.ProviderIMDb,
		movies: map[string][]model.SelectedMatchResult{
			norm("Валера"): {{Provider: model.ProviderIMDb, Kind: model.MediaKindMovie, OriginalTitle: "Валера", Year: 2003, IDs: model.ProviderTags{IMDbID: "ttru"}, ProviderReference: "ttru"}},
		},
	}
	_, err := runPipeline(t, root, false, fp)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	assertExists(t, filepath.Join(root, "Валера (2003) [imdbid-ttru].avi"))
}
