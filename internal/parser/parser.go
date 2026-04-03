package parser

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/davehornigan/MovieTagger/internal/model"
)

var (
	episodePatternSEx   = regexp.MustCompile(`(?i)\bS(\d{1,2})\s*E(\d{1,3})\b`)
	episodePatternSDotE = regexp.MustCompile(`(?i)\bS(\d{1,2})\s*[._-]\s*E(\d{1,3})\b`)
	episodePatternX     = regexp.MustCompile(`(?i)\b(\d{1,2})x(\d{1,3})\b`)
	yearPattern         = regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)

	imdbTagPattern = regexp.MustCompile(`(?i)\bimdbid[-_ ]?(tt\d{7,10})\b`)
	tmdbTagPattern = regexp.MustCompile(`(?i)\btmdbid[-_ ]?(\d+)\b`)

	// Supports "1.46Gb", "2GB", and variants.
	sizeTokenPattern   = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*g(?:b|ib)\b`)
	bracketsPattern    = regexp.MustCompile(`\[[^\]]*\]`)
	parenthesesPattern = regexp.MustCompile(`\([^)]*\)`)
	spacePattern       = regexp.MustCompile(`\s+`)
	separatorsPattern  = regexp.MustCompile(`[._]+`)
)

var releaseTokens = map[string]struct{}{
	"480p": {}, "720p": {}, "1080p": {}, "2160p": {},
	"bdrip": {}, "bluray": {}, "brrip": {}, "webrip": {}, "webdl": {}, "dvdrip": {}, "hdrip": {}, "remux": {},
	"x264": {}, "x265": {}, "h264": {}, "h265": {}, "hevc": {}, "av1": {},
	"aac": {}, "ac3": {}, "dts": {}, "ddp": {}, "mp3": {},
	"hdr": {}, "dv": {}, "dub": {}, "d": {}, "vo": {}, "mvo": {}, "avo": {},
	"megapeer": {}, "rarbg": {}, "yify": {},
}

var videoExtensions = map[string]struct{}{
	".mkv": {}, ".mp4": {}, ".avi": {}, ".mov": {}, ".wmv": {}, ".m4v": {}, ".ts": {},
}

type Parser struct{}

func New() *Parser {
	return &Parser{}
}

func (p *Parser) ParsePath(path string, isDir bool) model.ParsedFilenameInfo {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(base))
	nameNoExt := strings.TrimSuffix(base, filepath.Ext(base))

	info := model.ParsedFilenameInfo{
		Path:        path,
		Directory:   filepath.Dir(path),
		BaseName:    base,
		Extension:   ext,
		IsDirectory: isDir,
		IsVideoFile: !isDir && isVideoExtension(ext),
		Kind:        model.MediaKindUnknown,
	}

	fileTags := detectTags(nameNoExt)
	info.ExistingFileIDs = fileTags
	if fileTags.HasAny() && isEpisodeName(nameNoExt) {
		// For episode-like filenames, extracted IDs are episode-level hints.
		info.ExistingEpisodeIDs = fileTags
	}

	cleanName := stripTags(nameNoExt)
	info.YearHint = extractYear(cleanName)
	info.Episode = detectEpisode(cleanName)
	info.TitleHint = cleanTitle(cleanName, info.YearHint, info.Episode)

	switch {
	case info.Episode != nil:
		info.Kind = model.MediaKindEpisode
	case info.YearHint >= 1900 && info.YearHint <= 2099:
		info.Kind = model.MediaKindMovie
	default:
		info.Kind = model.MediaKindUnknown
	}

	return info
}

func detectTags(s string) model.ProviderTags {
	var tags model.ProviderTags

	if m := imdbTagPattern.FindStringSubmatch(s); len(m) == 2 {
		tags.IMDbID = strings.ToLower(m[1])
	}
	if m := tmdbTagPattern.FindStringSubmatch(s); len(m) == 2 {
		tags.TMDbID = m[1]
	}

	return tags
}

func stripTags(s string) string {
	s = imdbTagPattern.ReplaceAllString(s, " ")
	s = tmdbTagPattern.ReplaceAllString(s, " ")
	return s
}

func extractYear(s string) int {
	normalized := normalizeSeparators(s)
	m := yearPattern.FindStringSubmatch(normalized)
	if len(m) != 2 {
		return 0
	}
	year, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return year
}

func detectEpisode(s string) *model.EpisodeInfo {
	normalized := normalizeSeparators(s)

	if m := episodePatternSEx.FindStringSubmatch(normalized); len(m) == 3 {
		return buildEpisodeInfo(m[1], m[2], m[0])
	}
	if m := episodePatternSDotE.FindStringSubmatch(normalized); len(m) == 3 {
		return buildEpisodeInfo(m[1], m[2], m[0])
	}
	if m := episodePatternX.FindStringSubmatch(normalized); len(m) == 3 {
		return buildEpisodeInfo(m[1], m[2], m[0])
	}

	return nil
}

func buildEpisodeInfo(seasonRaw, episodeRaw, pattern string) *model.EpisodeInfo {
	season, errSeason := strconv.Atoi(seasonRaw)
	episode, errEpisode := strconv.Atoi(episodeRaw)
	if errSeason != nil || errEpisode != nil {
		return nil
	}
	return &model.EpisodeInfo{
		SeasonNumber:  season,
		EpisodeNumber: episode,
		Pattern:       pattern,
	}
}

func cleanTitle(s string, year int, episode *model.EpisodeInfo) string {
	working := s
	working = sizeTokenPattern.ReplaceAllString(working, " ")
	working = bracketsPattern.ReplaceAllString(working, " ")
	working = parenthesesPattern.ReplaceAllString(working, " ")
	working = normalizeSeparators(working)

	// Remove episode markers after normalization.
	working = episodePatternSEx.ReplaceAllString(working, " ")
	working = episodePatternSDotE.ReplaceAllString(working, " ")
	working = episodePatternX.ReplaceAllString(working, " ")

	if year >= 1900 && year <= 2099 {
		working = strings.ReplaceAll(working, strconv.Itoa(year), " ")
	}

	tokens := strings.Fields(working)
	kept := make([]string, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		cleanToken := normalizeToken(token)
		if cleanToken == "" {
			continue
		}
		if cleanToken == "web" && i+1 < len(tokens) && normalizeToken(tokens[i+1]) == "dl" {
			i++
			continue
		}
		if _, isRelease := releaseTokens[cleanToken]; isRelease {
			continue
		}
		kept = append(kept, token)
	}

	return strings.TrimSpace(spacePattern.ReplaceAllString(strings.Join(kept, " "), " "))
}

func normalizeSeparators(s string) string {
	s = separatorsPattern.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.TrimSpace(spacePattern.ReplaceAllString(s, " "))
	return s
}

func normalizeToken(token string) string {
	token = strings.ToLower(token)
	token = strings.ReplaceAll(token, "-", "")
	token = strings.ReplaceAll(token, "'", "")
	token = strings.ReplaceAll(token, "\"", "")
	token = strings.Trim(token, "[](){}.,;:!`")
	return token
}

func isVideoExtension(ext string) bool {
	_, ok := videoExtensions[strings.ToLower(ext)]
	return ok
}

func isEpisodeName(s string) bool {
	return detectEpisode(s) != nil
}
