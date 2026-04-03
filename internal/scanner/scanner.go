package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/davehornigan/MovieTagger/internal/config"
	"github.com/davehornigan/MovieTagger/internal/fsutil"
	"github.com/davehornigan/MovieTagger/internal/interactive"
	"github.com/davehornigan/MovieTagger/internal/logging"
	"github.com/davehornigan/MovieTagger/internal/matcher"
	"github.com/davehornigan/MovieTagger/internal/model"
	"github.com/davehornigan/MovieTagger/internal/parser"
	"github.com/davehornigan/MovieTagger/internal/planner"
	"github.com/davehornigan/MovieTagger/internal/providerfactory"
	"github.com/davehornigan/MovieTagger/internal/providers"
	"github.com/davehornigan/MovieTagger/internal/renamer"
)

type Options struct {
	DisableTMDB        bool
	DisableIMDb        bool
	NoInteractive      bool
	DryRun             bool
	Config             config.Config
	Logger             logging.Logger
	AvailableProviders []model.ProviderKind
	Parser             FilenameParser

	Selector  interactive.Selector
	Providers []providers.MetadataProvider
	Planner   planner.Service
	Executor  renamer.Executor
}

type Scanner struct {
	opts Options
}

type FilenameParser interface {
	ParsePath(path string, isDir bool) model.ParsedFilenameInfo
}

var (
	seasonDirPattern  = regexp.MustCompile(`^Season \d+$`)
	specialDirPattern = regexp.MustCompile(`^Special.*$`)
)

func New(opts Options) *Scanner {
	if opts.Parser == nil {
		opts.Parser = parser.New()
	}
	if opts.Logger == nil {
		opts.Logger = noopLogger{}
	}
	if !opts.NoInteractive && opts.Selector == nil {
		opts.Selector = interactive.NewTextSelector(nil, nil)
	}
	if opts.Planner == nil {
		opts.Planner = planner.New()
	}
	if opts.Executor == nil {
		opts.Executor = renamer.New(opts.Logger)
	}
	if len(opts.Providers) == 0 {
		opts.Providers = providerfactory.Build(providerfactory.BuildOptions{
			IMDbAPIKey:  opts.Config.IMDb.APIKey,
			TMDbAPIKey:  opts.Config.TMDb.APIKey,
			DisableIMDb: opts.DisableIMDb,
			DisableTMDb: opts.DisableTMDB,
			Logger:      opts.Logger,
		})
	}
	return &Scanner{opts: opts}
}

func (s *Scanner) Scan(ctx context.Context, scanDir string) (err error) {
	start := time.Now()
	s.opts.Logger.LogScanStart(scanDir)
	defer func() {
		s.opts.Logger.LogScanEnd(scanDir, time.Since(start), err)
	}()

	s.opts.Logger.Infof("scan options: dry_run=%t no_interactive=%t", s.opts.DryRun, s.opts.NoInteractive)
	s.opts.Logger.Infof("available providers: %v", s.opts.AvailableProviders)
	s.opts.Logger.Infof("loaded config path: %q", s.opts.Config.Path)

	scanResult, err := s.ScanResult(ctx, scanDir)
	if err != nil {
		return err
	}

	selection, matched, skipped, err := s.matchAll(ctx, scanResult)
	if err != nil {
		return err
	}

	plan, err := s.opts.Planner.BuildPlan(ctx, scanResult, selection, model.PlanOptions{DryRun: s.opts.DryRun})
	if err != nil {
		return fmt.Errorf("build plan: %w", err)
	}
	s.opts.Planner.ValidateCollisions(&plan)
	for _, c := range plan.Collisions {
		s.opts.Logger.LogCollision(c.TargetPath, c.SourcePaths)
	}

	report, err := s.opts.Executor.Execute(ctx, plan)
	if err != nil {
		return err
	}

	finalSkipped := skipped + report.Skipped
	summary := fmt.Sprintf(
		"summary: scanned=%d matched=%d renamed=%d skipped=%d invalid_structure=%d collisions=%d",
		len(scanResult.Items),
		matched,
		report.Applied,
		finalSkipped,
		len(scanResult.InvalidTVFindings),
		len(plan.Collisions),
	)
	s.opts.Logger.Infof(summary)
	_, _ = fmt.Fprintln(os.Stdout, summary)
	return nil
}

func (s *Scanner) matchAll(ctx context.Context, scanResult model.ScanResult) ([]model.SelectedItemMatch, int, int, error) {
	m := matcher.New(matcher.Options{
		NoInteractive: s.opts.NoInteractive,
		Selector:      s.opts.Selector,
		Logger:        s.opts.Logger,
	})

	selected := make([]model.SelectedItemMatch, 0)
	selectedByPath := map[string]model.SelectedMatchResult{}
	matchedCount := 0
	skippedCount := 0

	seriesItems, movieItems, episodeItems := splitItems(scanResult.Items)

	for _, item := range append(seriesItems, movieItems...) {
		candidates, err := s.queryCandidates(ctx, item, nil)
		if err != nil {
			return nil, 0, 0, err
		}
		out, err := m.Select(ctx, item, candidates)
		if err != nil {
			return nil, 0, 0, err
		}
		if out.Status == matcher.SelectionStatusSelected && out.Selected != nil {
			selected = append(selected, model.SelectedItemMatch{Path: item.Path, Match: *out.Selected})
			selectedByPath[item.Path] = *out.Selected
			matchedCount++
			s.opts.Logger.LogMatch(item.Path, *out.Selected)
			continue
		}
		skippedCount++
		s.opts.Logger.LogSkip(item.Path, string(out.Status))
	}

	for _, item := range episodeItems {
		seriesPath := seriesRootPathForEpisode(item.Path)
		seriesSelection, ok := selectedByPath[seriesPath]
		if !ok {
			skippedCount++
			s.opts.Logger.LogSkip(item.Path, "missing selected parent series")
			continue
		}

		candidates, err := s.queryCandidates(ctx, item, &seriesSelection)
		if err != nil {
			return nil, 0, 0, err
		}
		out, err := m.Select(ctx, item, candidates)
		if err != nil {
			return nil, 0, 0, err
		}
		if out.Status == matcher.SelectionStatusSelected && out.Selected != nil {
			selected = append(selected, model.SelectedItemMatch{Path: item.Path, Match: *out.Selected})
			matchedCount++
			s.opts.Logger.LogMatch(item.Path, *out.Selected)
			continue
		}
		skippedCount++
		s.opts.Logger.LogSkip(item.Path, string(out.Status))
	}

	return selected, matchedCount, skippedCount, nil
}

func (s *Scanner) queryCandidates(ctx context.Context, item model.ScanResultItem, seriesSelection *model.SelectedMatchResult) ([]model.SelectedMatchResult, error) {
	candidates := make([]model.SelectedMatchResult, 0)

	for _, provider := range s.opts.Providers {
		switch item.Kind {
		case model.MediaKindMovie:
			res, err := provider.MovieSeriesClient().SearchMovie(ctx, model.ProviderSearchCandidate{
				Provider:   provider.Kind(),
				Kind:       model.MediaKindMovie,
				QueryTitle: item.Parsed.TitleHint,
				QueryYear:  item.Parsed.YearHint,
			})
			if err != nil {
				return nil, fmt.Errorf("provider %s movie search failed: %w", provider.Kind(), err)
			}
			candidates = append(candidates, res...)
		case model.MediaKindSeries:
			queryTitle := item.Parsed.TitleHint
			if queryTitle == "" {
				queryTitle = filepath.Base(item.Path)
			}
			res, err := provider.MovieSeriesClient().SearchSeries(ctx, model.ProviderSearchCandidate{
				Provider:   provider.Kind(),
				Kind:       model.MediaKindSeries,
				QueryTitle: queryTitle,
				QueryYear:  item.Parsed.YearHint,
			})
			if err != nil {
				return nil, fmt.Errorf("provider %s series search failed: %w", provider.Kind(), err)
			}
			candidates = append(candidates, res...)
		case model.MediaKindEpisode:
			if item.Parsed.Episode == nil || seriesSelection == nil {
				continue
			}
			seriesForProvider, ok := adaptSeriesForProvider(*seriesSelection, provider.Kind())
			if !ok {
				continue
			}
			res, err := provider.EpisodeClient().LookupEpisode(ctx, seriesForProvider, *item.Parsed.Episode)
			if err != nil {
				return nil, fmt.Errorf("provider %s episode lookup failed: %w", provider.Kind(), err)
			}
			candidates = append(candidates, res)
		}
	}

	return candidates, nil
}

func adaptSeriesForProvider(series model.SelectedMatchResult, provider model.ProviderKind) (model.SelectedMatchResult, bool) {
	out := series
	switch provider {
	case model.ProviderIMDb:
		id := strings.TrimSpace(series.IDs.IMDbID)
		if id == "" && series.Provider == model.ProviderIMDb {
			id = strings.TrimSpace(series.ProviderReference)
		}
		if id == "" {
			return model.SelectedMatchResult{}, false
		}
		out.ProviderReference = id
		out.IDs.IMDbID = id
		return out, true
	case model.ProviderTMDb:
		id := strings.TrimSpace(series.IDs.TMDbID)
		if id == "" && series.Provider == model.ProviderTMDb {
			id = strings.TrimSpace(series.ProviderReference)
		}
		if id == "" {
			return model.SelectedMatchResult{}, false
		}
		out.ProviderReference = id
		out.IDs.TMDbID = id
		return out, true
	default:
		return model.SelectedMatchResult{}, false
	}
}

func splitItems(items []model.ScanResultItem) (series []model.ScanResultItem, movies []model.ScanResultItem, episodes []model.ScanResultItem) {
	for _, item := range items {
		switch item.Kind {
		case model.MediaKindSeries:
			if item.IsDir {
				series = append(series, item)
			}
		case model.MediaKindMovie:
			if !item.IsDir {
				movies = append(movies, item)
			}
		case model.MediaKindEpisode:
			if !item.IsDir {
				episodes = append(episodes, item)
			}
		}
	}
	sort.Slice(series, func(i, j int) bool { return series[i].Path < series[j].Path })
	sort.Slice(movies, func(i, j int) bool { return movies[i].Path < movies[j].Path })
	sort.Slice(episodes, func(i, j int) bool { return episodes[i].Path < episodes[j].Path })
	return series, movies, episodes
}

func seriesRootPathForEpisode(path string) string {
	return filepath.Dir(filepath.Dir(path))
}

func (s *Scanner) ScanResult(_ context.Context, scanDir string) (result model.ScanResult, err error) {
	rootAbs, err := filepath.Abs(scanDir)
	if err != nil {
		return model.ScanResult{}, fmt.Errorf("resolve scan dir: %w", err)
	}

	stat, err := os.Stat(rootAbs)
	if err != nil {
		return model.ScanResult{}, fmt.Errorf("stat scan dir: %w", err)
	}
	if !stat.IsDir() {
		return model.ScanResult{}, fmt.Errorf("scan path is not a directory: %s", scanDir)
	}

	result = model.ScanResult{RootPath: rootAbs}

	nodes, err := s.collectNodes(rootAbs)
	if err != nil {
		return model.ScanResult{}, err
	}

	validSeriesRoots := identifyValidSeriesRoots(nodes)

	s.appendSeriesAndSeasonItems(&result, nodes, validSeriesRoots)
	s.appendVideoItems(&result, nodes, validSeriesRoots)
	s.appendInvalidTVFindings(&result, nodes, validSeriesRoots)

	return result, nil
}

type scanNode struct {
	path   string
	isDir  bool
	parent string
	name   string
	parsed model.ParsedFilenameInfo
}

func (s *Scanner) collectNodes(root string) (map[string]scanNode, error) {
	nodes := map[string]scanNode{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		parent := filepath.Dir(path)
		parsed := s.opts.Parser.ParsePath(path, d.IsDir())
		nodes[path] = scanNode{
			path:   path,
			isDir:  d.IsDir(),
			parent: parent,
			name:   d.Name(),
			parsed: parsed,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk scan dir: %w", err)
	}

	return nodes, nil
}

func identifyValidSeriesRoots(nodes map[string]scanNode) map[string]struct{} {
	seasonByRoot := map[string][]string{}
	seasonHasEpisode := map[string]bool{}

	for path, node := range nodes {
		if !node.isDir {
			continue
		}
		if !isSeasonDirName(node.name) {
			continue
		}
		root := node.parent
		seasonByRoot[root] = append(seasonByRoot[root], path)
	}

	for seasonPath := range flattenSeasonSet(seasonByRoot) {
		for _, node := range nodes {
			if node.isDir || !node.parsed.IsVideoFile || node.parsed.Episode == nil {
				continue
			}
			if isInsideDir(node.path, seasonPath) {
				seasonHasEpisode[seasonPath] = true
			}
		}
	}

	valid := map[string]struct{}{}
	for root, seasons := range seasonByRoot {
		if len(seasons) == 0 {
			continue
		}
		if isSeasonDirName(filepath.Base(root)) {
			// A season folder cannot be a series root.
			continue
		}
		hasEpisode := false
		for _, seasonPath := range seasons {
			if seasonHasEpisode[seasonPath] {
				hasEpisode = true
				break
			}
		}
		if hasEpisode {
			valid[root] = struct{}{}
		}
	}

	return valid
}

func flattenSeasonSet(seasonByRoot map[string][]string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, seasons := range seasonByRoot {
		for _, season := range seasons {
			out[season] = struct{}{}
		}
	}
	return out
}

func (s *Scanner) appendSeriesAndSeasonItems(result *model.ScanResult, nodes map[string]scanNode, validSeriesRoots map[string]struct{}) {
	seriesPaths := make([]string, 0, len(validSeriesRoots))
	for root := range validSeriesRoots {
		seriesPaths = append(seriesPaths, root)
	}
	sort.Strings(seriesPaths)

	for _, root := range seriesPaths {
		rootNode := nodes[root]
		item := model.ScanResultItem{
			Path:   root,
			IsDir:  true,
			Kind:   model.MediaKindSeries,
			Parsed: rootNode.parsed,
			SeriesRoot: &model.SeriesRootInfo{
				Path:                root,
				NameHint:            filepath.Base(root),
				Valid:               true,
				DetectedSeasonPaths: immediateValidSeasonDirs(nodes, root),
			},
		}
		result.Items = append(result.Items, item)
	}

	for _, root := range seriesPaths {
		seasons := immediateValidSeasonDirs(nodes, root)
		for _, seasonPath := range seasons {
			seasonNode := nodes[seasonPath]
			seasonInfo := buildSeasonInfo(seasonNode)
			item := model.ScanResultItem{
				Path:   seasonPath,
				IsDir:  true,
				Kind:   model.MediaKindUnknown,
				Parsed: seasonNode.parsed,
				Season: &seasonInfo,
			}
			result.Items = append(result.Items, item)
		}
	}
}

func (s *Scanner) appendVideoItems(result *model.ScanResult, nodes map[string]scanNode, validSeriesRoots map[string]struct{}) {
	videoPaths := make([]string, 0)
	for path, node := range nodes {
		if node.isDir || !node.parsed.IsVideoFile {
			continue
		}
		videoPaths = append(videoPaths, path)
	}
	sort.Strings(videoPaths)

	for _, path := range videoPaths {
		node := nodes[path]
		parent := node.parent
		parentSeason := isSeasonDirName(filepath.Base(parent))
		seriesRoot := filepath.Dir(parent)
		_, parentSeriesValid := validSeriesRoots[seriesRoot]

		// Valid episode in strict hierarchy: series -> season -> episode.
		if node.parsed.Episode != nil && parentSeason && parentSeriesValid {
			item := model.ScanResultItem{
				Path:   path,
				IsDir:  false,
				Kind:   model.MediaKindEpisode,
				Parsed: node.parsed,
			}
			result.Items = append(result.Items, item)
			continue
		}

		// Movie-like processing for non-episode video files.
		if node.parsed.Episode == nil {
			item := model.ScanResultItem{
				Path:         path,
				IsDir:        false,
				Kind:         model.MediaKindMovie,
				Parsed:       node.parsed,
				RelatedFiles: s.relatedFilesForVideo(path, nodes),
			}
			result.Items = append(result.Items, item)
		}
	}
}

func (s *Scanner) relatedFilesForVideo(videoPath string, nodes map[string]scanNode) []string {
	videoDir := filepath.Dir(videoPath)
	siblings := make([]fsutil.PathEntry, 0)
	for _, node := range nodes {
		if node.parent != videoDir {
			continue
		}
		siblings = append(siblings, fsutil.PathEntry{Path: node.path, IsDir: node.isDir})
	}
	return fsutil.FindRelatedFiles(videoPath, false, siblings)
}

func (s *Scanner) appendInvalidTVFindings(result *model.ScanResult, nodes map[string]scanNode, validSeriesRoots map[string]struct{}) {
	findings := make([]model.InvalidTVFinding, 0)

	for path, node := range nodes {
		if node.isDir && isSeasonDirName(node.name) {
			if _, ok := validSeriesRoots[node.parent]; !ok {
				reason := "season directory is not under a valid series root"
				findings = append(findings, model.InvalidTVFinding{
					Type:   model.InvalidSeasonOutsideSeries,
					Path:   path,
					Reason: reason,
				})
				s.opts.Logger.LogInvalidSeriesStructure(path, reason)
			}
		}

		if node.isDir || !node.parsed.IsVideoFile || node.parsed.Episode == nil {
			continue
		}

		parent := node.parent
		if !isSeasonDirName(filepath.Base(parent)) {
			reason := "episode file is outside a season directory"
			findings = append(findings, model.InvalidTVFinding{
				Type:   model.InvalidEpisodeOutsideSeason,
				Path:   path,
				Reason: reason,
			})
			s.opts.Logger.LogInvalidSeriesStructure(path, reason)
			continue
		}

		seriesRoot := filepath.Dir(parent)
		if _, ok := validSeriesRoots[seriesRoot]; !ok {
			reason := "episode is inside a season directory but parent series is not valid"
			findings = append(findings, model.InvalidTVFinding{
				Type:   model.InvalidEpisodeInBadSeries,
				Path:   path,
				Reason: reason,
			})
			s.opts.Logger.LogInvalidSeriesStructure(path, reason)
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Type == findings[j].Type {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Type < findings[j].Type
	})
	result.InvalidTVFindings = findings
}

func immediateValidSeasonDirs(nodes map[string]scanNode, root string) []string {
	seasons := make([]string, 0)
	for path, node := range nodes {
		if !node.isDir || node.parent != root {
			continue
		}
		if isSeasonDirName(node.name) {
			seasons = append(seasons, path)
		}
	}
	sort.Strings(seasons)
	return seasons
}

func buildSeasonInfo(node scanNode) model.SeasonFolderInfo {
	info := model.SeasonFolderInfo{
		Path:      node.path,
		Name:      node.name,
		Valid:     true,
		IsSpecial: specialDirPattern.MatchString(node.name),
	}
	if seasonDirPattern.MatchString(node.name) {
		parts := strings.Split(node.name, " ")
		if len(parts) == 2 {
			// Best-effort extraction, already validated by regex.
			var n int
			fmt.Sscanf(parts[1], "%d", &n)
			info.SeasonNumber = n
		}
	}
	return info
}

func isInsideDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isSeasonDirName(name string) bool {
	return seasonDirPattern.MatchString(name) || specialDirPattern.MatchString(name)
}

type noopLogger struct{}

func (noopLogger) Infof(string, ...any)                                    {}
func (noopLogger) Warnf(string, ...any)                                    {}
func (noopLogger) Errorf(string, ...any)                                   {}
func (noopLogger) LogScanStart(string)                                     {}
func (noopLogger) LogScanEnd(string, time.Duration, error)                 {}
func (noopLogger) LogProviderCall(model.ProviderKind, string)              {}
func (noopLogger) LogProviderRetry(model.ProviderKind, string, int, error) {}
func (noopLogger) LogMatch(string, model.SelectedMatchResult)              {}
func (noopLogger) LogRenamePlan(model.RenamePlan)                          {}
func (noopLogger) LogSkip(string, string)                                  {}
func (noopLogger) LogCollision(string, []string)                           {}
func (noopLogger) LogInvalidSeriesStructure(string, string)                {}
func (noopLogger) Close() error                                            { return nil }
