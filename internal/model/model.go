package model

// MediaKind describes what kind of media object was detected.
type MediaKind string

const (
	MediaKindUnknown MediaKind = "unknown"
	MediaKindMovie   MediaKind = "movie"
	MediaKindSeries  MediaKind = "series"
	MediaKindEpisode MediaKind = "episode"
)

// ProviderKind identifies a metadata provider.
type ProviderKind string

const (
	ProviderIMDb ProviderKind = "imdb"
	ProviderTMDb ProviderKind = "tmdb"
)

// ProviderTags stores optional provider IDs.
// Partial tags are represented by setting only one field.
type ProviderTags struct {
	IMDbID string
	TMDbID string
}

func (t ProviderTags) HasAny() bool {
	return t.IMDbID != "" || t.TMDbID != ""
}

// EpisodeInfo is extracted from a filename and/or provider match.
type EpisodeInfo struct {
	SeasonNumber  int
	EpisodeNumber int
	Pattern       string
}

// SeasonFolderInfo describes a season folder candidate.
type SeasonFolderInfo struct {
	Path         string
	Name         string
	SeasonNumber int
	IsSpecial    bool
	Valid        bool
	Reason       string
}

// SeriesRootInfo describes a detected series root folder.
type SeriesRootInfo struct {
	Path                string
	NameHint            string
	Valid               bool
	Reason              string
	DetectedSeasonPaths []string
}

// ParsedFilenameInfo stores parser output for a single path.
type ParsedFilenameInfo struct {
	Path               string
	Directory          string
	BaseName           string
	Extension          string
	IsDirectory        bool
	IsVideoFile        bool
	Kind               MediaKind
	TitleHint          string
	YearHint           int
	Episode            *EpisodeInfo
	ExistingFileIDs    ProviderTags
	ExistingEpisodeIDs ProviderTags
}

// ProviderSearchCandidate is input for provider search operations.
type ProviderSearchCandidate struct {
	Provider   ProviderKind
	Kind       MediaKind
	QueryTitle string
	QueryYear  int
	Episode    *EpisodeInfo
}

// SelectedMatchResult is the resolved metadata chosen by planner/user.
type SelectedMatchResult struct {
	Provider      ProviderKind
	Kind          MediaKind
	Title         string
	OriginalTitle string
	Year          int
	Episode       *EpisodeInfo

	// IDs are for movie/series objects.
	IDs ProviderTags
	// EpisodeIDs must only contain episode-level IDs.
	EpisodeIDs ProviderTags

	ProviderReference string
	Confidence        float64
}

// ScanResultItem represents one standalone scanned object.
// Directories are always represented as standalone items.
type ScanResultItem struct {
	Path       string
	IsDir      bool
	Kind       MediaKind
	Parsed     ParsedFilenameInfo
	SeriesRoot *SeriesRootInfo
	Season     *SeasonFolderInfo

	// RelatedFiles must include only non-video files.
	RelatedFiles []string
}

// RenameOperationType defines how an operation should be applied.
type RenameOperationType string

const (
	RenameOpPrimaryFile RenameOperationType = "primary_file"
	RenameOpRelatedFile RenameOperationType = "related_file"
	RenameOpDirectory   RenameOperationType = "directory"
)

// RenameOperation is one atomic path rename action.
type RenameOperation struct {
	Type      RenameOperationType
	MediaKind MediaKind
	FromPath  string
	ToPath    string
	IsDir     bool
	RelatedTo string
}

// RenameCollision indicates two or more operations target the same path.
type RenameCollision struct {
	TargetPath  string
	SourcePaths []string
}

// RenamePlan is produced by planner and consumed by executor.
type RenamePlan struct {
	DryRun             bool
	Operations         []RenameOperation
	Collisions         []RenameCollision
	ValidationErrors   []string
	ValidationWarnings []string
}

func (p RenamePlan) HasBlockingIssues() bool {
	return len(p.Collisions) > 0 || len(p.ValidationErrors) > 0
}

// PlanOptions influences how a plan should be generated.
type PlanOptions struct {
	DryRun bool
}

type SelectedItemMatch struct {
	Path  string
	Match SelectedMatchResult
}

// ScanResult aggregates discovered items and non-fatal issues.
type ScanResult struct {
	RootPath          string
	Items             []ScanResultItem
	InvalidTVFindings []InvalidTVFinding
	Warnings          []string
}

type InvalidTVFindingType string

const (
	InvalidEpisodeOutsideSeason InvalidTVFindingType = "episode_outside_season"
	InvalidSeasonOutsideSeries  InvalidTVFindingType = "season_outside_series"
	InvalidEpisodeInBadSeries   InvalidTVFindingType = "episode_in_invalid_series"
)

type InvalidTVFinding struct {
	Type   InvalidTVFindingType
	Path   string
	Reason string
}
