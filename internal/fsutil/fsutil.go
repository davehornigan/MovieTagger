package fsutil

import (
	"path/filepath"
	"strings"
	"unicode"
)

var supportedVideoExtensions = map[string]struct{}{
	".mkv": {},
	".mp4": {},
	".avi": {},
	".mov": {},
	".wmv": {},
	".m4v": {},
	".ts":  {},
}

// IsVideoFile reports if path points to a supported video filename.
func IsVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := supportedVideoExtensions[ext]
	return ok
}

// PathEntry represents a path with known file-type information.
type PathEntry struct {
	Path  string
	IsDir bool
}

// FindRelatedFiles returns non-video files related to the provided video path.
// Rules:
// - same directory only
// - not a video file
// - basename contains exact video basename substring
func FindRelatedFiles(videoPath string, videoIsDir bool, siblings []PathEntry) []string {
	if videoIsDir {
		return nil
	}
	if !IsVideoFile(videoPath) {
		return nil
	}

	videoDir := filepath.Dir(videoPath)
	videoBase := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	if videoBase == "" {
		return nil
	}

	related := make([]string, 0, len(siblings))
	for _, candidate := range siblings {
		if candidate.IsDir {
			continue
		}
		if filepath.Dir(candidate.Path) != videoDir {
			continue
		}
		if IsVideoFile(candidate.Path) {
			// Video files are standalone and never related files.
			continue
		}

		candidateBase := filepath.Base(candidate.Path)
		if strings.Contains(candidateBase, videoBase) {
			related = append(related, candidate.Path)
		}
	}

	return related
}

// SanitizeTitleForFilesystem replaces unsafe filename characters with spaces.
// Unicode and character casing are preserved.
func SanitizeTitleForFilesystem(title string) string {
	unsafe := map[rune]struct{}{
		'/':  {},
		'\\': {},
		':':  {},
		'*':  {},
		'?':  {},
		'"':  {},
		'<':  {},
		'>':  {},
		'|':  {},
	}

	var b strings.Builder
	b.Grow(len(title))
	for _, r := range title {
		if _, replace := unsafe[r]; replace {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}

	return collapseWhitespace(b.String())
}

func collapseWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	prevSpace := true
	for _, r := range s {
		if unicode.IsSpace(r) {
			if prevSpace {
				continue
			}
			b.WriteRune(' ')
			prevSpace = true
			continue
		}

		b.WriteRune(r)
		prevSpace = false
	}

	return strings.TrimSpace(b.String())
}
