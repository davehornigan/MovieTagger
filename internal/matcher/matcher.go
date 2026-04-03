package matcher

import (
	"context"
	"errors"
	"sort"
	"strings"
	"unicode"

	"github.com/davehornigan/MovieTagger/internal/interactive"
	"github.com/davehornigan/MovieTagger/internal/logging"
	"github.com/davehornigan/MovieTagger/internal/model"
)

const (
	defaultAmbiguityDelta = 0.40
)

type SelectionStatus string

const (
	SelectionStatusSelected         SelectionStatus = "selected"
	SelectionStatusSkippedAmbiguous SelectionStatus = "skipped_ambiguous"
	SelectionStatusNoCandidates     SelectionStatus = "no_candidates"
)

type Options struct {
	NoInteractive     bool
	Selector          interactive.Selector
	Logger            logging.Logger
	PreferredProvider model.ProviderKind

	// AmbiguityDelta defines how close the top two scores can be and still be
	// considered ambiguous.
	AmbiguityDelta float64
}

type Matcher struct {
	opts Options
}

type ScoredCandidate struct {
	Candidate model.SelectedMatchResult
	Score     float64
}

type SelectionOutcome struct {
	Status    SelectionStatus
	Selected  *model.SelectedMatchResult
	Ambiguous bool
	Ranked    []ScoredCandidate
}

func New(opts Options) *Matcher {
	if opts.AmbiguityDelta <= 0 {
		opts.AmbiguityDelta = defaultAmbiguityDelta
	}
	if opts.PreferredProvider != model.ProviderIMDb && opts.PreferredProvider != model.ProviderTMDb {
		opts.PreferredProvider = model.ProviderTMDb
	}
	return &Matcher{opts: opts}
}

func (m *Matcher) Rank(item model.ScanResultItem, candidates []model.SelectedMatchResult) []ScoredCandidate {
	ranked := make([]ScoredCandidate, 0, len(candidates))
	for _, c := range candidates {
		score := ScoreCandidate(item, c)
		ranked = append(ranked, ScoredCandidate{Candidate: c, Score: score})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return providerRank(ranked[i].Candidate.Provider) < providerRank(ranked[j].Candidate.Provider)
		}
		return ranked[i].Score > ranked[j].Score
	})
	return ranked
}

func (m *Matcher) IsAmbiguous(ranked []ScoredCandidate) bool {
	if len(ranked) < 2 {
		return false
	}
	if ranked[0].Score < 0 {
		return true
	}
	delta := ranked[0].Score - ranked[1].Score
	return delta <= m.opts.AmbiguityDelta
}

func (m *Matcher) Select(ctx context.Context, item model.ScanResultItem, candidates []model.SelectedMatchResult) (SelectionOutcome, error) {
	ranked := m.Rank(item, candidates)
	if len(ranked) == 0 {
		return SelectionOutcome{
			Status: SelectionStatusNoCandidates,
			Ranked: ranked,
		}, nil
	}

	ambiguous := m.IsAmbiguous(ranked)
	if !ambiguous {
		selected := ranked[0].Candidate
		selected.Confidence = ranked[0].Score
		return SelectionOutcome{
			Status:    SelectionStatusSelected,
			Selected:  &selected,
			Ambiguous: false,
			Ranked:    ranked,
		}, nil
	}

	if selected, ok := m.preferOnePerProvider(candidates); ok {
		for _, r := range ranked {
			if r.Candidate.Provider == selected.Provider &&
				r.Candidate.ProviderReference == selected.ProviderReference &&
				r.Candidate.Title == selected.Title &&
				r.Candidate.Year == selected.Year {
				selected.Confidence = r.Score
				break
			}
		}
		return SelectionOutcome{
			Status:    SelectionStatusSelected,
			Selected:  &selected,
			Ambiguous: true,
			Ranked:    ranked,
		}, nil
	}

	if m.opts.NoInteractive || m.opts.Selector == nil {
		if m.opts.Logger != nil {
			m.opts.Logger.LogSkip(item.Path, "ambiguous match with no interactive selection")
		}
		return SelectionOutcome{
			Status:    SelectionStatusSkippedAmbiguous,
			Ambiguous: true,
			Ranked:    ranked,
		}, nil
	}

	choices := toCandidates(ranked)
	selected, err := m.opts.Selector.SelectMatch(ctx, item, choices)
	if err != nil {
		if errors.Is(err, interactive.ErrSkipSelection) {
			return SelectionOutcome{
				Status:    SelectionStatusSkippedAmbiguous,
				Ambiguous: true,
				Ranked:    ranked,
			}, nil
		}
		return SelectionOutcome{}, err
	}
	return SelectionOutcome{
		Status:    SelectionStatusSelected,
		Selected:  &selected,
		Ambiguous: true,
		Ranked:    ranked,
	}, nil
}

func (m *Matcher) preferOnePerProvider(candidates []model.SelectedMatchResult) (model.SelectedMatchResult, bool) {
	if len(candidates) != 2 {
		return model.SelectedMatchResult{}, false
	}
	var imdb *model.SelectedMatchResult
	var tmdb *model.SelectedMatchResult
	for i := range candidates {
		c := candidates[i]
		switch c.Provider {
		case model.ProviderIMDb:
			if imdb != nil {
				return model.SelectedMatchResult{}, false
			}
			imdb = &c
		case model.ProviderTMDb:
			if tmdb != nil {
				return model.SelectedMatchResult{}, false
			}
			tmdb = &c
		default:
			return model.SelectedMatchResult{}, false
		}
	}
	if imdb == nil || tmdb == nil {
		return model.SelectedMatchResult{}, false
	}
	if m.opts.PreferredProvider == model.ProviderIMDb {
		return *imdb, true
	}
	return *tmdb, true
}

func ScoreCandidate(item model.ScanResultItem, candidate model.SelectedMatchResult) float64 {
	score := 0.0

	// Provider priority: IMDb first, then TMDb.
	score += providerPriorityBonus(candidate.Provider)

	// Media kind must match strongly.
	if item.Kind == candidate.Kind {
		score += 4.0
	} else {
		score -= 6.0
	}

	// Title similarity is the dominant quality signal.
	score += scoreTitle(item.Parsed.TitleHint, candidate.Title) * 8.0

	// Year tolerance: exact best, +/-1 acceptable, larger deltas penalized.
	score += scoreYear(item.Parsed.YearHint, candidate.Year)

	// Episode coordinates for episode matching.
	score += scoreEpisode(item.Parsed.Episode, candidate.Episode)
	score += scoreKnownIDs(item.Parsed, candidate)

	return score
}

func scoreKnownIDs(parsed model.ParsedFilenameInfo, candidate model.SelectedMatchResult) float64 {
	score := 0.0
	score += scoreTagMatch(parsed.ExistingFileIDs.IMDbID, candidate.IDs.IMDbID, model.ProviderIMDb, candidate.Provider)
	score += scoreTagMatch(parsed.ExistingFileIDs.TMDbID, candidate.IDs.TMDbID, model.ProviderTMDb, candidate.Provider)
	score += scoreTagMatch(parsed.ExistingEpisodeIDs.IMDbID, candidate.EpisodeIDs.IMDbID, model.ProviderIMDb, candidate.Provider)
	score += scoreTagMatch(parsed.ExistingEpisodeIDs.TMDbID, candidate.EpisodeIDs.TMDbID, model.ProviderTMDb, candidate.Provider)
	return score
}

func scoreTagMatch(known string, candidateID string, candidateProvider model.ProviderKind, provider model.ProviderKind) float64 {
	known = strings.TrimSpace(strings.ToLower(known))
	if known == "" {
		return 0
	}
	candidateID = strings.TrimSpace(strings.ToLower(candidateID))
	if candidateID == "" {
		// Mild penalty when provider should know its own ID but does not return it.
		if provider == candidateProvider {
			return -2.0
		}
		return 0
	}
	if candidateID == known {
		return 20.0
	}
	// Strong mismatch penalty for conflicting known IDs.
	return -20.0
}

func scoreYear(localYear int, providerYear int) float64 {
	if localYear == 0 || providerYear == 0 {
		return 0
	}
	diff := localYear - providerYear
	if diff < 0 {
		diff = -diff
	}
	switch diff {
	case 0:
		return 3.0
	case 1:
		return 1.0
	default:
		return -4.0
	}
}

func scoreEpisode(local *model.EpisodeInfo, provider *model.EpisodeInfo) float64 {
	if local == nil {
		return 0
	}
	if provider == nil {
		return -2.0
	}

	score := 0.0
	if local.SeasonNumber == provider.SeasonNumber {
		score += 4.0
	} else {
		score -= 5.0
	}

	if local.EpisodeNumber == provider.EpisodeNumber {
		score += 6.0
	} else {
		score -= 7.0
	}

	return score
}

func scoreTitle(local string, remote string) float64 {
	localNorm := normalizeTitle(local)
	remoteNorm := normalizeTitle(remote)
	if localNorm == "" || remoteNorm == "" {
		return 0
	}
	if localNorm == remoteNorm {
		return 1.0
	}

	localTokens := strings.Fields(localNorm)
	remoteTokens := strings.Fields(remoteNorm)
	if len(localTokens) == 0 || len(remoteTokens) == 0 {
		return 0
	}

	intersection := 0
	seen := map[string]int{}
	for _, t := range localTokens {
		seen[t]++
	}
	for _, t := range remoteTokens {
		if seen[t] > 0 {
			intersection++
			seen[t]--
		}
	}

	union := len(localTokens) + len(remoteTokens) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := true
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if !prevSpace {
			b.WriteRune(' ')
		}
		prevSpace = true
	}
	return strings.TrimSpace(b.String())
}

func providerRank(kind model.ProviderKind) int {
	switch kind {
	case model.ProviderIMDb:
		return 0
	case model.ProviderTMDb:
		return 1
	default:
		return 99
	}
}

func providerPriorityBonus(kind model.ProviderKind) float64 {
	switch kind {
	case model.ProviderIMDb:
		return 0.15
	case model.ProviderTMDb:
		return 0.05
	default:
		return 0
	}
}

func toCandidates(ranked []ScoredCandidate) []model.SelectedMatchResult {
	out := make([]model.SelectedMatchResult, 0, len(ranked))
	for _, r := range ranked {
		c := r.Candidate
		c.Confidence = r.Score
		out = append(out, c)
	}
	return out
}
