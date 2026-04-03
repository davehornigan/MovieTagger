package logging

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/davehornigan/MovieTagger/internal/model"
)

// Logger defines the logging behavior used by services.
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)

	LogScanStart(scanDir string)
	LogScanEnd(scanDir string, duration time.Duration, err error)
	LogProviderCall(provider model.ProviderKind, operation string)
	LogProviderRetry(provider model.ProviderKind, operation string, attempt int, reason error)
	LogMatch(itemPath string, match model.SelectedMatchResult)
	LogRenamePlan(plan model.RenamePlan)
	LogSkip(path string, reason string)
	LogCollision(targetPath string, sourcePaths []string)
	LogInvalidSeriesStructure(path string, reason string)

	Close() error
}

type Options struct {
	LogFilePath string
}

type MultiLogger struct {
	mu   sync.Mutex
	file *os.File
}

func New(opts Options) (Logger, error) {
	logPath := opts.LogFilePath
	if logPath == "" {
		logPath = "movietagger.log"
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", logPath, err)
	}

	return &MultiLogger{file: file}, nil
}

func (l *MultiLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}
	return l.file.Close()
}

func (l *MultiLogger) Infof(format string, args ...any) {
	l.write("INFO", fmt.Sprintf(format, args...))
}

func (l *MultiLogger) Warnf(format string, args ...any) {
	l.write("WARN", fmt.Sprintf(format, args...))
}

func (l *MultiLogger) Errorf(format string, args ...any) {
	l.write("ERROR", fmt.Sprintf(format, args...))
}

func (l *MultiLogger) LogScanStart(scanDir string) {
	l.Infof("scan_start dir=%q", scanDir)
}

func (l *MultiLogger) LogScanEnd(scanDir string, duration time.Duration, err error) {
	if err != nil {
		l.Errorf("scan_end dir=%q duration=%s status=failed err=%v", scanDir, duration, err)
		return
	}
	l.Infof("scan_end dir=%q duration=%s status=ok", scanDir, duration)
}

func (l *MultiLogger) LogProviderCall(provider model.ProviderKind, operation string) {
	l.Infof("provider_call provider=%s operation=%q", provider, operation)
}

func (l *MultiLogger) LogProviderRetry(provider model.ProviderKind, operation string, attempt int, reason error) {
	l.Warnf("provider_retry provider=%s operation=%q attempt=%d reason=%v", provider, operation, attempt, reason)
}

func (l *MultiLogger) LogMatch(itemPath string, match model.SelectedMatchResult) {
	l.Infof(
		"match item=%q provider=%s kind=%s title=%q year=%d imdb_id=%q tmdb_id=%q episode_imdb_id=%q episode_tmdb_id=%q confidence=%.3f",
		itemPath,
		match.Provider,
		match.Kind,
		match.Title,
		match.Year,
		match.IDs.IMDbID,
		match.IDs.TMDbID,
		match.EpisodeIDs.IMDbID,
		match.EpisodeIDs.TMDbID,
		match.Confidence,
	)
}

func (l *MultiLogger) LogRenamePlan(plan model.RenamePlan) {
	l.Infof(
		"rename_plan dry_run=%t operations=%d collisions=%d errors=%d warnings=%d",
		plan.DryRun,
		len(plan.Operations),
		len(plan.Collisions),
		len(plan.ValidationErrors),
		len(plan.ValidationWarnings),
	)
}

func (l *MultiLogger) LogSkip(path string, reason string) {
	l.Warnf("skip path=%q reason=%q", path, reason)
}

func (l *MultiLogger) LogCollision(targetPath string, sourcePaths []string) {
	l.Warnf("collision target=%q sources=%v", targetPath, sourcePaths)
}

func (l *MultiLogger) LogInvalidSeriesStructure(path string, reason string) {
	l.Warnf("invalid_series_structure path=%q reason=%q", path, reason)
}

func (l *MultiLogger) write(level string, message string) {
	line := fmt.Sprintf("%s movietagger %-5s %s\n", time.Now().Format(time.RFC3339), level, message)

	l.mu.Lock()
	defer l.mu.Unlock()

	_, _ = os.Stderr.WriteString(line)
	if l.file != nil {
		_, _ = l.file.WriteString(line)
	}
}
