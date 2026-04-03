package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/davehornigan/MovieTagger/internal/config"
	"github.com/davehornigan/MovieTagger/internal/logging"
	"github.com/davehornigan/MovieTagger/internal/scanner"
)

const defaultConfigPath = "config.yaml"

func Run(args []string) error {
	if len(args) == 0 {
		return usageError("missing command")
	}

	switch args[0] {
	case "scan":
		return runScan(args[1:])
	default:
		return usageError(fmt.Sprintf("unknown command %q", args[0]))
	}
}

func runScan(args []string) error {
	if len(args) == 0 {
		return usageError("scan requires SCAN-DIR")
	}

	scanDir := args[0]
	flagArgs := args[1:]

	fs := flag.NewFlagSet("scan", flag.ContinueOnError)

	var disableTMDB bool
	var disableIMDb bool
	var noInteractive bool
	var dryRun bool
	var configPath string
	var logFilePath string

	fs.BoolVar(&disableTMDB, "disable-tmdb", false, "Disable TMDb provider")
	fs.BoolVar(&disableIMDb, "disable-imdb", false, "Disable IMDb provider")
	fs.BoolVar(&noInteractive, "no-interactive", false, "Disable interactive mode")
	fs.BoolVar(&dryRun, "dry-run", false, "Preview changes without renaming")
	fs.StringVar(&configPath, "config", defaultConfigPath, "Path to YAML config file")
	fs.StringVar(&logFilePath, "log-file", "movietagger.log", "Path to log file")

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	if fs.NArg() != 0 {
		return usageError("unexpected extra arguments for scan")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, err := logging.New(logging.Options{LogFilePath: logFilePath})
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() {
		_ = logger.Close()
	}()

	availability := config.ResolveProviderAvailability(cfg, disableIMDb, disableTMDB)
	if !availability.HasAnyAvailable() {
		for _, reason := range availability.UnavailableReasons() {
			logger.Warnf(reason)
		}
		return fmt.Errorf("no available providers: enable at least one provider and provide its api_key")
	}

	if !availability.IMDb.Available {
		logger.Warnf("imdb unavailable: %s", availability.IMDb.Reason)
	}
	if !availability.TMDb.Available {
		logger.Warnf("tmdb unavailable: %s", availability.TMDb.Reason)
	}

	s := scanner.New(scanner.Options{
		DisableTMDB:        disableTMDB,
		DisableIMDb:        disableIMDb,
		NoInteractive:      noInteractive,
		DryRun:             dryRun,
		Config:             cfg,
		Logger:             logger,
		AvailableProviders: availability.AvailableKinds(),
	})

	return s.Scan(context.Background(), scanDir)
}

func usageError(msg string) error {
	usage := "movietagger scan SCAN-DIR [--disable-tmdb] [--disable-imdb] [--no-interactive] [--dry-run]"
	return fmt.Errorf("%s\nusage: %s", strings.TrimSpace(msg), usage)
}
