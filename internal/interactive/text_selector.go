package interactive

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/davehornigan/MovieTagger/internal/model"
)

// TextSelector provides plain-text interactive candidate selection.
// Prompting is serialized to stay safe when called from concurrent workflows.
type TextSelector struct {
	mu  sync.Mutex
	in  *bufio.Reader
	out io.Writer
}

func NewTextSelector(in io.Reader, out io.Writer) *TextSelector {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	return &TextSelector{
		in:  bufio.NewReader(in),
		out: out,
	}
}

func (s *TextSelector) SelectMatch(ctx context.Context, item model.ScanResultItem, candidates []model.SelectedMatchResult) (model.SelectedMatchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(candidates) == 0 {
		return model.SelectedMatchResult{}, fmt.Errorf("no candidates to select from")
	}

	name := filepath.Base(item.Path)
	_, _ = fmt.Fprintf(s.out, "\nAmbiguous match for: %s\n", name)
	_, _ = fmt.Fprintf(s.out, "Path: %s\n", item.Path)
	_, _ = fmt.Fprintf(s.out, "Parsed: kind=%s title=%q year=%d", item.Parsed.Kind, item.Parsed.TitleHint, item.Parsed.YearHint)
	if item.Parsed.Episode != nil {
		_, _ = fmt.Fprintf(s.out, " season=%d episode=%d", item.Parsed.Episode.SeasonNumber, item.Parsed.Episode.EpisodeNumber)
	}
	_, _ = fmt.Fprintln(s.out)

	for i, c := range candidates {
		_, _ = fmt.Fprintf(s.out, "%d) type=%s title=%q", i+1, c.Kind, c.Title)
		if c.OriginalTitle != "" && c.OriginalTitle != c.Title {
			_, _ = fmt.Fprintf(s.out, " original=%q", c.OriginalTitle)
		}
		if c.Year > 0 {
			_, _ = fmt.Fprintf(s.out, " year=%d", c.Year)
		}
		_, _ = fmt.Fprintf(s.out, " source=%s", c.Provider)

		ids := formatIDs(c)
		if ids != "" {
			_, _ = fmt.Fprintf(s.out, " ids=%s", ids)
		}
		_, _ = fmt.Fprintln(s.out)
	}

	for {
		select {
		case <-ctx.Done():
			return model.SelectedMatchResult{}, ctx.Err()
		default:
		}

		_, _ = fmt.Fprintf(s.out, "Choose candidate [1-%d] or 's' to skip: ", len(candidates))
		line, err := s.in.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return model.SelectedMatchResult{}, ErrSkipSelection
			}
			return model.SelectedMatchResult{}, err
		}

		choice := strings.TrimSpace(strings.ToLower(line))
		if choice == "s" || choice == "skip" {
			return model.SelectedMatchResult{}, ErrSkipSelection
		}

		n, err := strconv.Atoi(choice)
		if err != nil || n < 1 || n > len(candidates) {
			_, _ = fmt.Fprintln(s.out, "Invalid choice. Enter a candidate number or 's' to skip.")
			continue
		}
		return candidates[n-1], nil
	}
}

func (s *TextSelector) ConfirmPlan(ctx context.Context, plan model.RenamePlan) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	_, _ = fmt.Fprintf(
		s.out,
		"Apply plan with %d operations (dry-run=%t, collisions=%d)? [y/N]: ",
		len(plan.Operations),
		plan.DryRun,
		len(plan.Collisions),
	)
	line, err := s.in.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}

func formatIDs(c model.SelectedMatchResult) string {
	parts := make([]string, 0, 4)
	if c.IDs.IMDbID != "" {
		parts = append(parts, "imdbid-"+c.IDs.IMDbID)
	}
	if c.IDs.TMDbID != "" {
		parts = append(parts, "tmdbid-"+c.IDs.TMDbID)
	}
	if c.EpisodeIDs.IMDbID != "" {
		parts = append(parts, "episode-imdbid-"+c.EpisodeIDs.IMDbID)
	}
	if c.EpisodeIDs.TMDbID != "" {
		parts = append(parts, "episode-tmdbid-"+c.EpisodeIDs.TMDbID)
	}
	return strings.Join(parts, ",")
}
