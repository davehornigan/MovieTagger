package planner

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/davehornigan/MovieTagger/internal/fsutil"
	"github.com/davehornigan/MovieTagger/internal/model"
)

// Service builds and validates rename plans from scan outputs and selected metadata.
type Service interface {
	BuildPlan(ctx context.Context, scan model.ScanResult, selected []model.SelectedItemMatch, opts model.PlanOptions) (model.RenamePlan, error)
	ValidateCollisions(plan *model.RenamePlan)
}

type Planner struct{}

func New() *Planner {
	return &Planner{}
}

func (p *Planner) BuildPlan(_ context.Context, scan model.ScanResult, selected []model.SelectedItemMatch, opts model.PlanOptions) (model.RenamePlan, error) {
	plan := model.RenamePlan{
		DryRun: opts.DryRun,
	}
	selection := selectionByPath(selected)
	seasonWidths := episodeWidthBySeriesSeason(scan.Items)

	for _, item := range scan.Items {
		match, ok := selection[item.Path]
		if !ok {
			continue
		}

		switch {
		case item.IsDir && item.Kind == model.MediaKindSeries:
			op, changed := buildSeriesDirRename(item, match)
			if !changed {
				plan.ValidationWarnings = append(plan.ValidationWarnings, fmt.Sprintf("no-op skipped: %s", item.Path))
				continue
			}
			plan.Operations = append(plan.Operations, op)
		case !item.IsDir && item.Kind == model.MediaKindMovie:
			ops, warnings := buildMovieRenames(item, match)
			plan.Operations = append(plan.Operations, ops...)
			plan.ValidationWarnings = append(plan.ValidationWarnings, warnings...)
		case !item.IsDir && item.Kind == model.MediaKindEpisode:
			ops, warnings := buildEpisodeRenames(item, match, seasonWidths)
			plan.Operations = append(plan.Operations, ops...)
			plan.ValidationWarnings = append(plan.ValidationWarnings, warnings...)
		}
	}

	p.ValidateCollisions(&plan)
	return plan, nil
}

func (p *Planner) ValidateCollisions(plan *model.RenamePlan) {
	targets := map[string][]string{}
	for _, op := range plan.Operations {
		targets[op.ToPath] = append(targets[op.ToPath], op.FromPath)
	}
	plan.Collisions = plan.Collisions[:0]
	for target, sources := range targets {
		if len(sources) <= 1 {
			continue
		}
		sort.Strings(sources)
		plan.Collisions = append(plan.Collisions, model.RenameCollision{
			TargetPath:  target,
			SourcePaths: sources,
		})
	}
	sort.Slice(plan.Collisions, func(i, j int) bool { return plan.Collisions[i].TargetPath < plan.Collisions[j].TargetPath })
	if len(plan.Collisions) > 0 {
		plan.ValidationErrors = append(plan.ValidationErrors, fmt.Sprintf("detected %d collision(s)", len(plan.Collisions)))
	}
}

func selectionByPath(selected []model.SelectedItemMatch) map[string]model.SelectedMatchResult {
	out := make(map[string]model.SelectedMatchResult, len(selected))
	for _, s := range selected {
		out[s.Path] = s.Match
	}
	return out
}

func buildMovieRenames(item model.ScanResultItem, match model.SelectedMatchResult) ([]model.RenameOperation, []string) {
	ops := make([]model.RenameOperation, 0, 1+len(item.RelatedFiles))
	warnings := make([]string, 0)

	fromPath := item.Path
	dir := filepath.Dir(fromPath)
	ext := filepath.Ext(fromPath)
	oldBase := strings.TrimSuffix(filepath.Base(fromPath), ext)

	title := fsutil.SanitizeTitleForFilesystem(providerTitle(match, item.Parsed.TitleHint))
	year := providerYear(match, item.Parsed.YearHint)
	fileIDs := mergeProviderTags(item.Parsed.ExistingFileIDs, match.IDs)

	newBase := buildMovieBase(title, year, fileIDs)
	toPath := filepath.Join(dir, newBase+ext)
	if toPath == fromPath {
		warnings = append(warnings, fmt.Sprintf("no-op skipped: %s", fromPath))
		return ops, warnings
	}

	ops = append(ops, model.RenameOperation{
		Type:      model.RenameOpPrimaryFile,
		MediaKind: model.MediaKindMovie,
		FromPath:  fromPath,
		ToPath:    toPath,
	})

	for _, rel := range item.RelatedFiles {
		relBase := filepath.Base(rel)
		if !strings.Contains(relBase, oldBase) {
			continue
		}
		newRelBase := strings.Replace(relBase, oldBase, newBase, 1)
		newRelPath := filepath.Join(filepath.Dir(rel), newRelBase)
		if newRelPath == rel {
			continue
		}
		ops = append(ops, model.RenameOperation{
			Type:      model.RenameOpRelatedFile,
			MediaKind: model.MediaKindMovie,
			FromPath:  rel,
			ToPath:    newRelPath,
			RelatedTo: fromPath,
		})
	}
	return ops, warnings
}

func buildSeriesDirRename(item model.ScanResultItem, match model.SelectedMatchResult) (model.RenameOperation, bool) {
	fromPath := item.Path
	dir := filepath.Dir(fromPath)

	title := fsutil.SanitizeTitleForFilesystem(providerTitle(match, filepath.Base(fromPath)))
	year := providerYear(match, item.Parsed.YearHint)
	ids := mergeProviderTags(item.Parsed.ExistingFileIDs, match.IDs)

	newName := buildSeriesBase(title, year, ids)
	toPath := filepath.Join(dir, newName)
	if toPath == fromPath {
		return model.RenameOperation{}, false
	}
	return model.RenameOperation{
		Type:      model.RenameOpDirectory,
		MediaKind: model.MediaKindSeries,
		FromPath:  fromPath,
		ToPath:    toPath,
		IsDir:     true,
	}, true
}

func buildEpisodeRenames(item model.ScanResultItem, match model.SelectedMatchResult, widthBySeason map[string]int) ([]model.RenameOperation, []string) {
	ops := make([]model.RenameOperation, 0, 1+len(item.RelatedFiles))
	warnings := make([]string, 0)

	if item.Parsed.Episode == nil {
		warnings = append(warnings, fmt.Sprintf("episode skipped (missing parsed episode): %s", item.Path))
		return ops, warnings
	}

	epIDs := mergeProviderTags(item.Parsed.ExistingEpisodeIDs, match.EpisodeIDs)
	if !epIDs.HasAny() {
		warnings = append(warnings, fmt.Sprintf("episode skipped (missing episode-level ids): %s", item.Path))
		return ops, warnings
	}

	fromPath := item.Path
	dir := filepath.Dir(fromPath)
	ext := filepath.Ext(fromPath)
	oldBase := strings.TrimSuffix(filepath.Base(fromPath), ext)

	seriesTitle := fsutil.SanitizeTitleForFilesystem(providerTitle(match, item.Parsed.TitleHint))
	season := item.Parsed.Episode.SeasonNumber
	episode := item.Parsed.Episode.EpisodeNumber
	seasonKey := seasonGroupKey(item.Path, season)
	epWidth := widthBySeason[seasonKey]
	if epWidth == 0 {
		epWidth = 2
	}

	newBase := buildEpisodeBase(seriesTitle, season, episode, epWidth, epIDs)
	toPath := filepath.Join(dir, newBase+ext)
	if toPath == fromPath {
		warnings = append(warnings, fmt.Sprintf("no-op skipped: %s", fromPath))
		return ops, warnings
	}

	ops = append(ops, model.RenameOperation{
		Type:      model.RenameOpPrimaryFile,
		MediaKind: model.MediaKindEpisode,
		FromPath:  fromPath,
		ToPath:    toPath,
	})

	for _, rel := range item.RelatedFiles {
		relBase := filepath.Base(rel)
		if !strings.Contains(relBase, oldBase) {
			continue
		}
		newRelBase := strings.Replace(relBase, oldBase, newBase, 1)
		newRelPath := filepath.Join(filepath.Dir(rel), newRelBase)
		if newRelPath == rel {
			continue
		}
		ops = append(ops, model.RenameOperation{
			Type:      model.RenameOpRelatedFile,
			MediaKind: model.MediaKindEpisode,
			FromPath:  rel,
			ToPath:    newRelPath,
			RelatedTo: fromPath,
		})
	}
	return ops, warnings
}

func episodeWidthBySeriesSeason(items []model.ScanResultItem) map[string]int {
	maxBySeason := map[string]int{}
	for _, item := range items {
		if item.IsDir || item.Kind != model.MediaKindEpisode || item.Parsed.Episode == nil {
			continue
		}
		key := seasonGroupKey(item.Path, item.Parsed.Episode.SeasonNumber)
		if item.Parsed.Episode.EpisodeNumber > maxBySeason[key] {
			maxBySeason[key] = item.Parsed.Episode.EpisodeNumber
		}
	}

	width := map[string]int{}
	for key, maxEpisode := range maxBySeason {
		if maxEpisode >= 100 {
			width[key] = 3
			continue
		}
		width[key] = 2
	}
	return width
}

func seasonGroupKey(path string, season int) string {
	seasonDir := filepath.Dir(path)
	seriesDir := filepath.Dir(seasonDir)
	return seriesDir + "::" + strconv.Itoa(season)
}

func buildMovieBase(title string, year int, ids model.ProviderTags) string {
	parts := []string{title}
	if year > 0 {
		parts = append(parts, fmt.Sprintf("(%d)", year))
	}
	if ids.IMDbID != "" {
		parts = append(parts, "[imdbid-"+ids.IMDbID+"]")
	}
	if ids.TMDbID != "" {
		parts = append(parts, "[tmdbid-"+ids.TMDbID+"]")
	}
	return strings.Join(parts, " ")
}

func buildSeriesBase(title string, year int, ids model.ProviderTags) string {
	return buildMovieBase(title, year, ids)
}

func buildEpisodeBase(seriesTitle string, season int, episode int, epWidth int, ids model.ProviderTags) string {
	epFmt := "%02d"
	if epWidth >= 3 {
		epFmt = "%03d"
	}
	parts := []string{
		seriesTitle,
		fmt.Sprintf("S%02dE"+epFmt, season, episode),
	}
	// Episodes require tmdb then imdb order per spec.
	if ids.TMDbID != "" {
		parts = append(parts, "[tmdbid-"+ids.TMDbID+"]")
	}
	if ids.IMDbID != "" {
		parts = append(parts, "[imdbid-"+ids.IMDbID+"]")
	}
	return strings.Join(parts, " ")
}

func mergeProviderTags(existing model.ProviderTags, fromProvider model.ProviderTags) model.ProviderTags {
	out := existing
	if out.IMDbID == "" {
		out.IMDbID = fromProvider.IMDbID
	}
	if out.TMDbID == "" {
		out.TMDbID = fromProvider.TMDbID
	}
	return out
}

func providerTitle(match model.SelectedMatchResult, fallback string) string {
	if strings.TrimSpace(match.OriginalTitle) != "" {
		return strings.TrimSpace(match.OriginalTitle)
	}
	if strings.TrimSpace(match.Title) != "" {
		return strings.TrimSpace(match.Title)
	}
	return strings.TrimSpace(fallback)
}

func providerYear(match model.SelectedMatchResult, fallback int) int {
	if match.Year > 0 {
		return match.Year
	}
	return fallback
}
