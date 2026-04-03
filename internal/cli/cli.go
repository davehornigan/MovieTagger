package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/davehornigan/MovieTagger/internal/config"
	"github.com/davehornigan/MovieTagger/internal/logging"
	"github.com/davehornigan/MovieTagger/internal/scanner"
)

const defaultConfigPath = "config.yaml"

var Version = "dev"

func Run(args []string) error {
	if len(args) == 0 {
		printRootHelp()
		return usageError("missing command")
	}

	if isHelpArg(args[0]) || args[0] == "help" {
		printRootHelp()
		return nil
	}
	if isVersionArg(args[0]) || args[0] == "version" {
		fmt.Fprintf(os.Stdout, "movietagger %s\n", Version)
		return nil
	}

	switch args[0] {
	case "scan":
		return runScan(args[1:])
	default:
		printRootHelp()
		return usageError(fmt.Sprintf("unknown command %q", args[0]))
	}
}

func runScan(args []string) error {
	if len(args) == 1 && isHelpArg(args[0]) {
		printScanHelp()
		return nil
	}

	if len(args) == 0 {
		printScanHelp()
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
	usage := `movietagger scan SCAN-DIR [--disable-tmdb] [--disable-imdb] [--no-interactive] [--dry-run] [--config PATH] [--log-file PATH]`
	return fmt.Errorf("%s\nusage: %s", strings.TrimSpace(msg), usage)
}

func printRootHelp() {
	fmt.Fprintln(os.Stdout, "MovieTagger CLI")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Usage:")
	fmt.Fprintln(os.Stdout, "  movietagger scan SCAN-DIR [--disable-tmdb] [--disable-imdb] [--no-interactive] [--dry-run] [--config PATH] [--log-file PATH]")
	fmt.Fprintln(os.Stdout, "  movietagger --version")
	fmt.Fprintln(os.Stdout, "  movietagger --help")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Options:")
	fmt.Fprintln(os.Stdout, "  --help, -h       Show help")
	fmt.Fprintln(os.Stdout, "  --version, -v    Show version")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Scan options:")
	fmt.Fprintln(os.Stdout, "  --disable-tmdb   Disable TMDb provider")
	fmt.Fprintln(os.Stdout, "  --disable-imdb   Disable IMDb provider")
	fmt.Fprintln(os.Stdout, "  --no-interactive Disable interactive candidate selection")
	fmt.Fprintln(os.Stdout, "  --dry-run        Build and validate plan without filesystem changes")
	fmt.Fprintln(os.Stdout, "  --config PATH    Path to YAML config file (default: config.yaml)")
	fmt.Fprintln(os.Stdout, "  --log-file PATH  Path to log file (default: movietagger.log)")
}

func printScanHelp() {
	fmt.Fprintln(os.Stdout, "Usage:")
	fmt.Fprintln(os.Stdout, "  movietagger scan SCAN-DIR [--disable-tmdb] [--disable-imdb] [--no-interactive] [--dry-run] [--config PATH] [--log-file PATH]")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Options:")
	fmt.Fprintln(os.Stdout, "  --disable-tmdb   Disable TMDb provider")
	fmt.Fprintln(os.Stdout, "  --disable-imdb   Disable IMDb provider")
	fmt.Fprintln(os.Stdout, "  --no-interactive Disable interactive candidate selection")
	fmt.Fprintln(os.Stdout, "  --dry-run        Build and validate plan without filesystem changes")
	fmt.Fprintln(os.Stdout, "  --config PATH    Path to YAML config file (default: config.yaml)")
	fmt.Fprintln(os.Stdout, "  --log-file PATH  Path to log file (default: movietagger.log)")
}

func isHelpArg(arg string) bool {
	return arg == "--help" || arg == "-h"
}

func isVersionArg(arg string) bool {
	return arg == "--version" || arg == "-v"
}
