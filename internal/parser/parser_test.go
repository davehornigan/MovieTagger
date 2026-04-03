package parser

import (
	"testing"

	"github.com/davehornigan/MovieTagger/internal/model"
)

func TestParsePath_MovieAndEpisodeExamples(t *testing.T) {
	p := New()

	tests := []struct {
		name        string
		input       string
		wantKind    model.MediaKind
		wantTitle   string
		wantYear    int
		wantSeason  int
		wantEpisode int
	}{
		{
			name:      "clean movie",
			input:     "Valera (2003).mp4",
			wantKind:  model.MediaKindMovie,
			wantTitle: "Valera",
			wantYear:  2003,
		},
		{
			name:      "simple movie with trailing year",
			input:     "The Movie 2000.mkv",
			wantKind:  model.MediaKindMovie,
			wantTitle: "The Movie",
			wantYear:  2000,
		},
		{
			name:      "movie with edition note",
			input:     "The Movie (2000) [Director's Cut].avi",
			wantKind:  model.MediaKindMovie,
			wantTitle: "The Movie",
			wantYear:  2000,
		},
		{
			name:      "release style movie",
			input:     "L.Amour.et.les.Forets.2023.D.BDRip.1.46Gb.MegaPeer.avi",
			wantKind:  model.MediaKindMovie,
			wantTitle: "L Amour et les Forets",
			wantYear:  2023,
		},
		{
			name:        "release style episode underscore",
			input:       "Some_Show_S01E02_1080p_WEB-DL.mkv",
			wantKind:    model.MediaKindEpisode,
			wantTitle:   "Some Show",
			wantYear:    0,
			wantSeason:  1,
			wantEpisode: 2,
		},
		{
			name:        "release style episode 1x",
			input:       "Another.Show.1x03.720p.HDTV.x265.mkv",
			wantKind:    model.MediaKindEpisode,
			wantTitle:   "Another Show HDTV",
			wantYear:    0,
			wantSeason:  1,
			wantEpisode: 3,
		},
		{
			name:      "cyrillic movie",
			input:     "Валера.2003.BDRip.avi",
			wantKind:  model.MediaKindMovie,
			wantTitle: "Валера",
			wantYear:  2003,
		},
		{
			name:        "cyrillic episode",
			input:       "Мой.Сериал.S01E02.WEBRip.mkv",
			wantKind:    model.MediaKindEpisode,
			wantTitle:   "Мой Сериал",
			wantYear:    0,
			wantSeason:  1,
			wantEpisode: 2,
		},
		{
			name:        "episode with dotted marker",
			input:       "My.Show.S01.E02.mkv",
			wantKind:    model.MediaKindEpisode,
			wantTitle:   "My Show",
			wantYear:    0,
			wantSeason:  1,
			wantEpisode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.ParsePath(tt.input, false)

			if got.Kind != tt.wantKind {
				t.Fatalf("kind mismatch: got %s want %s", got.Kind, tt.wantKind)
			}
			if got.TitleHint != tt.wantTitle {
				t.Fatalf("title mismatch: got %q want %q", got.TitleHint, tt.wantTitle)
			}
			if got.YearHint != tt.wantYear {
				t.Fatalf("year mismatch: got %d want %d", got.YearHint, tt.wantYear)
			}

			if tt.wantKind == model.MediaKindEpisode {
				if got.Episode == nil {
					t.Fatalf("expected episode info, got nil")
				}
				if got.Episode.SeasonNumber != tt.wantSeason {
					t.Fatalf("season mismatch: got %d want %d", got.Episode.SeasonNumber, tt.wantSeason)
				}
				if got.Episode.EpisodeNumber != tt.wantEpisode {
					t.Fatalf("episode mismatch: got %d want %d", got.Episode.EpisodeNumber, tt.wantEpisode)
				}
			}
		})
	}
}

func TestParsePath_TagDetectionAndPartialTagging(t *testing.T) {
	p := New()

	tests := []struct {
		name            string
		input           string
		wantIMDbID      string
		wantTMDbID      string
		wantEpisodeIMDb string
		wantEpisodeTMDb string
		wantKind        model.MediaKind
	}{
		{
			name:       "movie imdb tag only partial",
			input:      "Valera (2003) [imdbid-tt1234567].mp4",
			wantIMDbID: "tt1234567",
			wantKind:   model.MediaKindMovie,
		},
		{
			name:       "movie tmdb tag only partial",
			input:      "Valera (2003) [tmdbid-98765].mp4",
			wantTMDbID: "98765",
			wantKind:   model.MediaKindMovie,
		},
		{
			name:       "movie both tags",
			input:      "Valera (2003) [imdbid-tt1234567] [tmdbid-98765].mp4",
			wantIMDbID: "tt1234567",
			wantTMDbID: "98765",
			wantKind:   model.MediaKindMovie,
		},
		{
			name:            "episode tags are mirrored to episode ids",
			input:           "My.Show.S01E02.[imdbid-tt7654321].[tmdbid-555].mkv",
			wantIMDbID:      "tt7654321",
			wantTMDbID:      "555",
			wantEpisodeIMDb: "tt7654321",
			wantEpisodeTMDb: "555",
			wantKind:        model.MediaKindEpisode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.ParsePath(tt.input, false)

			if got.Kind != tt.wantKind {
				t.Fatalf("kind mismatch: got %s want %s", got.Kind, tt.wantKind)
			}

			if got.ExistingFileIDs.IMDbID != tt.wantIMDbID {
				t.Fatalf("file imdb id mismatch: got %q want %q", got.ExistingFileIDs.IMDbID, tt.wantIMDbID)
			}
			if got.ExistingFileIDs.TMDbID != tt.wantTMDbID {
				t.Fatalf("file tmdb id mismatch: got %q want %q", got.ExistingFileIDs.TMDbID, tt.wantTMDbID)
			}

			if got.ExistingEpisodeIDs.IMDbID != tt.wantEpisodeIMDb {
				t.Fatalf("episode imdb id mismatch: got %q want %q", got.ExistingEpisodeIDs.IMDbID, tt.wantEpisodeIMDb)
			}
			if got.ExistingEpisodeIDs.TMDbID != tt.wantEpisodeTMDb {
				t.Fatalf("episode tmdb id mismatch: got %q want %q", got.ExistingEpisodeIDs.TMDbID, tt.wantEpisodeTMDb)
			}
		})
	}
}

func TestParsePath_VideoAndNonVideo(t *testing.T) {
	p := New()

	video := p.ParsePath("movie.mkv", false)
	if !video.IsVideoFile {
		t.Fatalf("expected mkv to be video")
	}

	nonVideo := p.ParsePath("subtitle.srt", false)
	if nonVideo.IsVideoFile {
		t.Fatalf("expected srt to be non-video")
	}
}
