package planner

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davehornigan/MovieTagger/internal/model"
)

func TestBuildPlan_MoviePlan(t *testing.T) {
	p := New()
	scan := model.ScanResult{
		Items: []model.ScanResultItem{
			{
				Path: "/media/Valera (2003).mp4",
				Kind: model.MediaKindMovie,
				Parsed: model.ParsedFilenameInfo{
					YearHint: 2003,
				},
			},
		},
	}
	selected := []model.SelectedItemMatch{
		{
			Path: "/media/Valera (2003).mp4",
			Match: model.SelectedMatchResult{
				Kind:          model.MediaKindMovie,
				OriginalTitle: "Valera",
				Year:          2003,
				IDs: model.ProviderTags{
					IMDbID: "tt1234567",
					TMDbID: "98765",
				},
			},
		},
	}
	plan, err := p.BuildPlan(context.Background(), scan, selected, model.PlanOptions{DryRun: true})
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	if len(plan.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(plan.Operations))
	}
	want := "/media/Valera (2003) [imdbid-tt1234567] [tmdbid-98765].mp4"
	if plan.Operations[0].ToPath != want {
		t.Fatalf("unexpected movie target: %s", plan.Operations[0].ToPath)
	}
}

func TestBuildPlan_PartiallyTaggedMovieEnrichment(t *testing.T) {
	p := New()
	path := "/media/Valera (2003) [imdbid-tt1234567].mp4"
	scan := model.ScanResult{
		Items: []model.ScanResultItem{
			{
				Path: path,
				Kind: model.MediaKindMovie,
				Parsed: model.ParsedFilenameInfo{
					YearHint: 2003,
					ExistingFileIDs: model.ProviderTags{
						IMDbID: "tt1234567",
					},
				},
			},
		},
	}
	selected := []model.SelectedItemMatch{
		{
			Path: path,
			Match: model.SelectedMatchResult{
				Kind:          model.MediaKindMovie,
				OriginalTitle: "Valera",
				Year:          2003,
				IDs: model.ProviderTags{
					IMDbID: "tt1234567",
					TMDbID: "98765",
				},
			},
		},
	}

	plan, err := p.BuildPlan(context.Background(), scan, selected, model.PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	if len(plan.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(plan.Operations))
	}
	to := plan.Operations[0].ToPath
	if strings.Count(to, "[imdbid-tt1234567]") != 1 {
		t.Fatalf("imdb tag duplicated: %s", to)
	}
	if !strings.Contains(to, "[tmdbid-98765]") {
		t.Fatalf("missing tmdb enrichment: %s", to)
	}
}

func TestBuildPlan_SeriesDirectoryPlan(t *testing.T) {
	p := New()
	path := "/media/My Show"
	scan := model.ScanResult{
		Items: []model.ScanResultItem{
			{
				Path:   path,
				IsDir:  true,
				Kind:   model.MediaKindSeries,
				Parsed: model.ParsedFilenameInfo{},
			},
		},
	}
	selected := []model.SelectedItemMatch{
		{
			Path: path,
			Match: model.SelectedMatchResult{
				Kind:          model.MediaKindSeries,
				OriginalTitle: "My Show",
				Year:          2011,
				IDs: model.ProviderTags{
					IMDbID: "tt0944947",
					TMDbID: "1399",
				},
			},
		},
	}
	plan, err := p.BuildPlan(context.Background(), scan, selected, model.PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	if len(plan.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(plan.Operations))
	}
	want := "/media/My Show (2011) [imdbid-tt0944947] [tmdbid-1399]"
	if plan.Operations[0].ToPath != want {
		t.Fatalf("unexpected series dir target: %s", plan.Operations[0].ToPath)
	}
}

func TestBuildPlan_EpisodePlan(t *testing.T) {
	p := New()
	path := "/media/My Show/Season 1/file.mkv"
	scan := model.ScanResult{
		Items: []model.ScanResultItem{
			{
				Path: path,
				Kind: model.MediaKindEpisode,
				Parsed: model.ParsedFilenameInfo{
					TitleHint: "My Show",
					Episode: &model.EpisodeInfo{
						SeasonNumber:  1,
						EpisodeNumber: 2,
					},
				},
			},
			{
				Path: "/media/My Show/Season 1/other.S01E100.mkv",
				Kind: model.MediaKindEpisode,
				Parsed: model.ParsedFilenameInfo{
					Episode: &model.EpisodeInfo{
						SeasonNumber:  1,
						EpisodeNumber: 100,
					},
				},
			},
		},
	}
	selected := []model.SelectedItemMatch{
		{
			Path: path,
			Match: model.SelectedMatchResult{
				Kind:          model.MediaKindEpisode,
				OriginalTitle: "My Show",
				EpisodeIDs: model.ProviderTags{
					TMDbID: "55",
					IMDbID: "tt000055",
				},
			},
		},
	}
	plan, err := p.BuildPlan(context.Background(), scan, selected, model.PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	if len(plan.Operations) != 1 {
		t.Fatalf("expected 1 op, got %d", len(plan.Operations))
	}
	want := "/media/My Show/Season 1/My Show S01E002 [tmdbid-55] [imdbid-tt000055].mkv"
	if plan.Operations[0].ToPath != want {
		t.Fatalf("unexpected episode target: %s", plan.Operations[0].ToPath)
	}
}

func TestBuildPlan_RelatedSubtitlePosterRename(t *testing.T) {
	p := New()
	video := "/media/Movie.mp4"
	scan := model.ScanResult{
		Items: []model.ScanResultItem{
			{
				Path: video,
				Kind: model.MediaKindMovie,
				Parsed: model.ParsedFilenameInfo{
					YearHint: 2000,
				},
				RelatedFiles: []string{
					"/media/Movie.srt",
					"/media/Movie-cover.jpg",
				},
			},
		},
	}
	selected := []model.SelectedItemMatch{
		{
			Path: video,
			Match: model.SelectedMatchResult{
				OriginalTitle: "The Movie",
				Year:          2000,
				IDs: model.ProviderTags{
					IMDbID: "tt123",
				},
			},
		},
	}
	plan, err := p.BuildPlan(context.Background(), scan, selected, model.PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	if len(plan.Operations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(plan.Operations))
	}
	var relatedCount int
	for _, op := range plan.Operations {
		if op.Type == model.RenameOpRelatedFile {
			relatedCount++
		}
	}
	if relatedCount != 2 {
		t.Fatalf("expected 2 related ops, got %d", relatedCount)
	}
}

func TestBuildPlan_Collisions(t *testing.T) {
	p := New()
	scan := model.ScanResult{
		Items: []model.ScanResultItem{
			{Path: "/media/A.mp4", Kind: model.MediaKindMovie, Parsed: model.ParsedFilenameInfo{YearHint: 2000}},
			{Path: "/media/B.mp4", Kind: model.MediaKindMovie, Parsed: model.ParsedFilenameInfo{YearHint: 2000}},
		},
	}
	selected := []model.SelectedItemMatch{
		{Path: "/media/A.mp4", Match: model.SelectedMatchResult{OriginalTitle: "Same", Year: 2000}},
		{Path: "/media/B.mp4", Match: model.SelectedMatchResult{OriginalTitle: "Same", Year: 2000}},
	}
	plan, err := p.BuildPlan(context.Background(), scan, selected, model.PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	if len(plan.Collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(plan.Collisions))
	}
}

func TestBuildPlan_NoOpHandling(t *testing.T) {
	p := New()
	path := "/media/Valera (2003) [imdbid-tt1].mp4"
	scan := model.ScanResult{
		Items: []model.ScanResultItem{
			{
				Path: path,
				Kind: model.MediaKindMovie,
				Parsed: model.ParsedFilenameInfo{
					YearHint: 2003,
					ExistingFileIDs: model.ProviderTags{
						IMDbID: "tt1",
					},
				},
			},
		},
	}
	selected := []model.SelectedItemMatch{
		{
			Path: path,
			Match: model.SelectedMatchResult{
				OriginalTitle: "Valera",
				Year:          2003,
				IDs: model.ProviderTags{
					IMDbID: "tt1",
				},
			},
		},
	}
	plan, err := p.BuildPlan(context.Background(), scan, selected, model.PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	if len(plan.Operations) != 0 {
		t.Fatalf("expected no operation for no-op rename, got %d", len(plan.Operations))
	}
	if len(plan.ValidationWarnings) == 0 {
		t.Fatalf("expected no-op warning")
	}
}

func TestBuildPlan_InvalidCharacterSanitization(t *testing.T) {
	p := New()
	path := "/media/Movie.mp4"
	scan := model.ScanResult{
		Items: []model.ScanResultItem{
			{Path: path, Kind: model.MediaKindMovie},
		},
	}
	selected := []model.SelectedItemMatch{
		{
			Path: path,
			Match: model.SelectedMatchResult{
				OriginalTitle: `Bad/Title:*? "Name"|`,
				Year:          2001,
			},
		},
	}
	plan, err := p.BuildPlan(context.Background(), scan, selected, model.PlanOptions{})
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	if len(plan.Operations) != 1 {
		t.Fatalf("expected one op, got %d", len(plan.Operations))
	}
	base := filepath.Base(plan.Operations[0].ToPath)
	if strings.ContainsAny(base, `/\:*?"<>|`) {
		t.Fatalf("found invalid filesystem chars in %q", base)
	}
}
