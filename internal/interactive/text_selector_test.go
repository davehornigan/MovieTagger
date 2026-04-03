package interactive

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/davehornigan/MovieTagger/internal/model"
)

func TestTextSelector_SelectByNumber(t *testing.T) {
	in := strings.NewReader("2\n")
	var out strings.Builder
	s := NewTextSelector(in, &out)

	item := model.ScanResultItem{
		Path: "/media/The.Movie.2000.mkv",
		Parsed: model.ParsedFilenameInfo{
			Kind:      model.MediaKindMovie,
			TitleHint: "The Movie",
			YearHint:  2000,
		},
	}
	candidates := []model.SelectedMatchResult{
		{Provider: model.ProviderIMDb, Kind: model.MediaKindMovie, Title: "Movie A", Year: 2000},
		{Provider: model.ProviderTMDb, Kind: model.MediaKindMovie, Title: "Movie B", Year: 2000},
	}

	got, err := s.SelectMatch(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("SelectMatch error: %v", err)
	}
	if got.Title != "Movie B" {
		t.Fatalf("expected second candidate, got %q", got.Title)
	}
	if !strings.Contains(out.String(), "Choose candidate [1-2] or 's' to skip") {
		t.Fatalf("expected prompt output, got: %s", out.String())
	}
}

func TestTextSelector_Skip(t *testing.T) {
	in := strings.NewReader("s\n")
	var out strings.Builder
	s := NewTextSelector(in, &out)

	_, err := s.SelectMatch(context.Background(), model.ScanResultItem{Path: "/x/file.mkv"}, []model.SelectedMatchResult{
		{Provider: model.ProviderIMDb, Kind: model.MediaKindMovie, Title: "A"},
	})
	if !errors.Is(err, ErrSkipSelection) {
		t.Fatalf("expected ErrSkipSelection, got %v", err)
	}
}

func TestTextSelector_InvalidThenValidInput(t *testing.T) {
	in := strings.NewReader("abc\n9\n1\n")
	var out strings.Builder
	s := NewTextSelector(in, &out)

	candidates := []model.SelectedMatchResult{
		{Provider: model.ProviderIMDb, Kind: model.MediaKindMovie, Title: "A"},
		{Provider: model.ProviderTMDb, Kind: model.MediaKindMovie, Title: "B"},
	}
	got, err := s.SelectMatch(context.Background(), model.ScanResultItem{Path: "/x/file.mkv"}, candidates)
	if err != nil {
		t.Fatalf("SelectMatch error: %v", err)
	}
	if got.Title != "A" {
		t.Fatalf("expected first candidate, got %q", got.Title)
	}

	output := out.String()
	if strings.Count(output, "Invalid choice.") < 2 {
		t.Fatalf("expected invalid choice feedback, output: %s", output)
	}
}
