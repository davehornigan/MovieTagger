package matcher

import (
	"context"
	"testing"

	"github.com/davehornigan/MovieTagger/internal/interactive"
	"github.com/davehornigan/MovieTagger/internal/model"
)

func TestSelect_ExactYearMatchWins(t *testing.T) {
	m := New(Options{NoInteractive: true})
	item := movieItem("The Movie", 2000)

	candidates := []model.SelectedMatchResult{
		movieCandidate(model.ProviderTMDb, "The Movie", 2001),
		movieCandidate(model.ProviderIMDb, "The Movie", 2000),
	}

	out, err := m.Select(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if out.Status != SelectionStatusSelected || out.Selected == nil {
		t.Fatalf("expected selected outcome, got %+v", out)
	}
	if out.Selected.Year != 2000 {
		t.Fatalf("expected exact year match, got %d", out.Selected.Year)
	}
}

func TestSelect_YearPlusMinusOneAllowed(t *testing.T) {
	m := New(Options{NoInteractive: true})
	item := movieItem("The Movie", 2000)

	candidates := []model.SelectedMatchResult{
		movieCandidate(model.ProviderTMDb, "The Movie", 2001),
		movieCandidate(model.ProviderIMDb, "The Movie", 2003),
	}

	out, err := m.Select(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if out.Selected == nil {
		t.Fatalf("expected selected candidate")
	}
	if out.Selected.Year != 2001 {
		t.Fatalf("expected +/-1 year candidate to win, got %d", out.Selected.Year)
	}
}

func TestSelect_YearDifferenceGreaterThanOnePenalized(t *testing.T) {
	m := New(Options{NoInteractive: true})
	item := movieItem("The Movie", 2000)

	candidates := []model.SelectedMatchResult{
		movieCandidate(model.ProviderIMDb, "The Movie", 2004),
		movieCandidate(model.ProviderTMDb, "The Movie", 2001),
	}

	out, err := m.Select(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if out.Selected == nil {
		t.Fatalf("expected selected candidate")
	}
	if out.Selected.Year != 2001 {
		t.Fatalf("expected year 2001 to win over >1 diff, got %d", out.Selected.Year)
	}
}

func TestSelect_AmbiguousNoInteractiveSkips(t *testing.T) {
	m := New(Options{
		NoInteractive:  true,
		AmbiguityDelta: 0.5,
	})
	item := movieItem("The Movie", 0)

	candidates := []model.SelectedMatchResult{
		movieCandidate(model.ProviderIMDb, "The Movie", 0),
		movieCandidate(model.ProviderTMDb, "The Movie", 0),
		movieCandidate(model.ProviderIMDb, "The Movie", 0),
	}

	out, err := m.Select(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if out.Status != SelectionStatusSkippedAmbiguous {
		t.Fatalf("expected ambiguous skip, got %s", out.Status)
	}
	if !out.Ambiguous {
		t.Fatalf("expected ambiguous=true")
	}
}

func TestSelect_ClearWinnerAutoSelected(t *testing.T) {
	m := New(Options{NoInteractive: true})
	item := movieItem("The Matrix", 1999)

	candidates := []model.SelectedMatchResult{
		movieCandidate(model.ProviderTMDb, "Random Documentary", 2017),
		movieCandidate(model.ProviderIMDb, "The Matrix", 1999),
	}

	out, err := m.Select(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if out.Status != SelectionStatusSelected || out.Selected == nil {
		t.Fatalf("expected selected outcome")
	}
	if out.Selected.Title != "The Matrix" {
		t.Fatalf("expected clear winner title, got %q", out.Selected.Title)
	}
}

func TestSelect_EpisodeCandidateMatching(t *testing.T) {
	m := New(Options{NoInteractive: true})
	item := model.ScanResultItem{
		Path: "/media/Show/Season 1/Show.S01E02.mkv",
		Kind: model.MediaKindEpisode,
		Parsed: model.ParsedFilenameInfo{
			TitleHint: "Show",
			Episode: &model.EpisodeInfo{
				SeasonNumber:  1,
				EpisodeNumber: 2,
			},
		},
	}

	candidates := []model.SelectedMatchResult{
		{
			Provider: model.ProviderTMDb,
			Kind:     model.MediaKindEpisode,
			Title:    "Show",
			Episode: &model.EpisodeInfo{
				SeasonNumber:  1,
				EpisodeNumber: 3,
			},
		},
		{
			Provider: model.ProviderIMDb,
			Kind:     model.MediaKindEpisode,
			Title:    "Show",
			Episode: &model.EpisodeInfo{
				SeasonNumber:  1,
				EpisodeNumber: 2,
			},
		},
	}

	out, err := m.Select(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if out.Selected == nil {
		t.Fatalf("expected selected episode candidate")
	}
	if out.Selected.Episode == nil || out.Selected.Episode.EpisodeNumber != 2 {
		t.Fatalf("expected matching episode candidate to win")
	}
}

func TestSelect_AmbiguousUsesInteractiveSelector(t *testing.T) {
	selector := &fakeSelector{
		selected: movieCandidate(model.ProviderTMDb, "The Movie", 2000),
	}
	m := New(Options{
		NoInteractive:  false,
		Selector:       selector,
		AmbiguityDelta: 0.5,
	})
	item := movieItem("The Movie", 0)
	candidates := []model.SelectedMatchResult{
		movieCandidate(model.ProviderIMDb, "The Movie", 0),
		movieCandidate(model.ProviderTMDb, "The Movie", 0),
		movieCandidate(model.ProviderIMDb, "The Movie", 0),
	}

	out, err := m.Select(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if !selector.called {
		t.Fatalf("expected interactive selector to be called")
	}
	if out.Selected == nil || out.Selected.Provider != model.ProviderTMDb {
		t.Fatalf("expected selected candidate from fake selector")
	}
}

func TestSelect_AmbiguousInteractiveSkip(t *testing.T) {
	selector := &fakeSelector{
		err: interactive.ErrSkipSelection,
	}
	m := New(Options{
		NoInteractive:  false,
		Selector:       selector,
		AmbiguityDelta: 0.5,
	})
	item := movieItem("The Movie", 0)
	candidates := []model.SelectedMatchResult{
		movieCandidate(model.ProviderIMDb, "The Movie", 0),
		movieCandidate(model.ProviderTMDb, "The Movie", 0),
		movieCandidate(model.ProviderIMDb, "The Movie", 0),
	}

	out, err := m.Select(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if out.Status != SelectionStatusSkippedAmbiguous {
		t.Fatalf("expected skipped ambiguous, got %s", out.Status)
	}
}

func TestSelect_OneCandidatePerProvider_UsesPreferredTMDbByDefault(t *testing.T) {
	m := New(Options{NoInteractive: true})
	item := movieItem("Same", 2020)
	candidates := []model.SelectedMatchResult{
		movieCandidate(model.ProviderIMDb, "Same", 2020),
		movieCandidate(model.ProviderTMDb, "Same", 2020),
	}
	out, err := m.Select(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if out.Selected == nil {
		t.Fatalf("expected selected candidate")
	}
	if out.Selected.Provider != model.ProviderTMDb {
		t.Fatalf("expected tmdb preferred by default, got %s", out.Selected.Provider)
	}
}

func TestSelect_OneCandidatePerProvider_UsesConfiguredPreferredProvider(t *testing.T) {
	m := New(Options{
		NoInteractive:     true,
		PreferredProvider: model.ProviderIMDb,
	})
	item := movieItem("Same", 2020)
	candidates := []model.SelectedMatchResult{
		movieCandidate(model.ProviderIMDb, "Same", 2020),
		movieCandidate(model.ProviderTMDb, "Same", 2020),
	}
	out, err := m.Select(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if out.Selected == nil {
		t.Fatalf("expected selected candidate")
	}
	if out.Selected.Provider != model.ProviderIMDb {
		t.Fatalf("expected imdb preferred, got %s", out.Selected.Provider)
	}
}

func TestSelect_KnownIDWinsOverProviderPriority(t *testing.T) {
	m := New(Options{NoInteractive: true, PreferredProvider: model.ProviderIMDb})
	item := movieItem("Countdown", 1982)
	item.Parsed.ExistingFileIDs = model.ProviderTags{TMDbID: "3863"}

	candidates := []model.SelectedMatchResult{
		{
			Provider: model.ProviderIMDb,
			Kind:     model.MediaKindSeries,
			Title:    "Countdown",
			Year:     1982,
			IDs:      model.ProviderTags{IMDbID: "tt0138228"},
		},
		{
			Provider: model.ProviderTMDb,
			Kind:     model.MediaKindSeries,
			Title:    "Countdown",
			Year:     1982,
			IDs:      model.ProviderTags{TMDbID: "3863"},
		},
	}

	out, err := m.Select(context.Background(), item, candidates)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if out.Selected == nil {
		t.Fatalf("expected selected candidate")
	}
	if out.Selected.Provider != model.ProviderTMDb {
		t.Fatalf("expected tmdb candidate with known id to win, got %s", out.Selected.Provider)
	}
}

type fakeSelector struct {
	called   bool
	selected model.SelectedMatchResult
	err      error
}

func (f *fakeSelector) SelectMatch(_ context.Context, _ model.ScanResultItem, _ []model.SelectedMatchResult) (model.SelectedMatchResult, error) {
	f.called = true
	if f.err != nil {
		return model.SelectedMatchResult{}, f.err
	}
	return f.selected, nil
}

func (f *fakeSelector) ConfirmPlan(_ context.Context, _ model.RenamePlan) (bool, error) {
	return true, nil
}

func movieItem(title string, year int) model.ScanResultItem {
	return model.ScanResultItem{
		Path: "/media/test.mkv",
		Kind: model.MediaKindMovie,
		Parsed: model.ParsedFilenameInfo{
			TitleHint: title,
			YearHint:  year,
		},
	}
}

func movieCandidate(provider model.ProviderKind, title string, year int) model.SelectedMatchResult {
	return model.SelectedMatchResult{
		Provider: provider,
		Kind:     model.MediaKindMovie,
		Title:    title,
		Year:     year,
	}
}
