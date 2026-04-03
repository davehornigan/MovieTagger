# MovieTagger

MovieTagger is a Go CLI application skeleton for scanning media directories and preparing safe rename plans using IMDb and TMDb metadata.

## Command usage

```bash
movietagger scan SCAN-DIR [--disable-tmdb] [--disable-imdb] [--no-interactive] [--dry-run]
```

Optional runtime flags:

- `--config PATH` (default: `config.yaml`)
- `--log-file PATH` (default: `movietagger.log`)

## Sample config

```yaml
priority_provider: tmdb # allowed: tmdb | imdb, default: tmdb

imdb:
  api_key: "your-imdb-api-key"
tmdb:
  api_key: "your-tmdb-api-key"
```

A sample file is also available at `config.example.yaml`.

## Current status

- CLI parsing and startup wiring are implemented.
- YAML config loading is implemented.
- Provider availability validation is implemented.
- Console and file logging are implemented (thread-safe).
- Scanner and provider network logic are intentionally not implemented yet.
