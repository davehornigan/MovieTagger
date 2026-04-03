package imdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/davehornigan/MovieTagger/internal/logging"
	"github.com/davehornigan/MovieTagger/internal/model"
	"github.com/davehornigan/MovieTagger/internal/providers"
)

const defaultBaseURL = "https://www.omdbapi.com/"

type Options struct {
	APIKey      string
	BaseURL     string
	HTTPClient  *http.Client
	Logger      logging.Logger
	RetryCount  int
	BaseBackoff time.Duration
	Sleep       func(time.Duration)
}

type Client struct {
	apiKey      string
	baseURL     string
	httpClient  *http.Client
	logger      logging.Logger
	retryCount  int
	baseBackoff time.Duration
	sleep       func(time.Duration)
}

func NewClient(opts Options) *Client {
	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	retries := opts.RetryCount
	if retries <= 0 {
		retries = 3
	}
	backoff := opts.BaseBackoff
	if backoff <= 0 {
		backoff = 200 * time.Millisecond
	}

	return &Client{
		apiKey:      strings.TrimSpace(opts.APIKey),
		baseURL:     strings.TrimRight(baseURL, "/"),
		httpClient:  httpClient,
		logger:      opts.Logger,
		retryCount:  retries,
		baseBackoff: backoff,
		sleep:       opts.Sleep,
	}
}

func (c *Client) SearchMovie(ctx context.Context, candidate model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error) {
	return c.search(ctx, "search_movie", candidate.QueryTitle, "movie")
}

func (c *Client) SearchSeries(ctx context.Context, candidate model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error) {
	return c.search(ctx, "search_series", candidate.QueryTitle, "series")
}

func (c *Client) ResolveByKnownIDs(ctx context.Context, candidate model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error) {
	imdbID := strings.TrimSpace(candidate.KnownIDs.IMDbID)
	if imdbID == "" {
		return []model.SelectedMatchResult{}, nil
	}

	operation := "resolve_by_known_ids"
	if c.logger != nil {
		c.logger.LogProviderCall(model.ProviderIMDb, operation)
	}

	result, err := providers.DoWithRetry(
		ctx,
		c.logger,
		model.ProviderIMDb,
		operation,
		c.retryCount,
		c.baseBackoff,
		c.sleep,
		func() (model.SelectedMatchResult, error) {
			query := url.Values{}
			query.Set("apikey", c.apiKey)
			query.Set("i", imdbID)

			var resp omdbTitleResponse
			if err := c.get(ctx, query, &resp); err != nil {
				return model.SelectedMatchResult{}, err
			}
			if !resp.IsSuccess() {
				if isNotFoundAPIError(resp.Error) {
					return model.SelectedMatchResult{}, nil
				}
				return model.SelectedMatchResult{}, fmt.Errorf("imdb api: %s", resp.Error)
			}

			kind := parseOMDbType(resp.Type)
			if candidate.Kind != model.MediaKindUnknown && kind != candidate.Kind {
				return model.SelectedMatchResult{}, nil
			}

			title := strings.TrimSpace(resp.Title)
			if title == "" {
				return model.SelectedMatchResult{}, nil
			}
			return model.SelectedMatchResult{
				Provider:      model.ProviderIMDb,
				Kind:          kind,
				Title:         title,
				OriginalTitle: title,
				Year:          parseYear(resp.Year),
				IDs: model.ProviderTags{
					IMDbID: strings.TrimSpace(resp.IMDbID),
				},
				ProviderReference: strings.TrimSpace(resp.IMDbID),
			}, nil
		},
	)
	if err != nil {
		return nil, err
	}
	if result.Title == "" && !result.IDs.HasAny() {
		return []model.SelectedMatchResult{}, nil
	}
	return []model.SelectedMatchResult{result}, nil
}

func (c *Client) LookupEpisode(ctx context.Context, series model.SelectedMatchResult, episode model.EpisodeInfo) (model.SelectedMatchResult, error) {
	seriesID := series.ProviderReference
	if seriesID == "" {
		seriesID = series.IDs.IMDbID
	}
	if seriesID == "" {
		return model.SelectedMatchResult{}, fmt.Errorf("imdb episode lookup requires series imdb id")
	}

	operation := "lookup_episode"
	if c.logger != nil {
		c.logger.LogProviderCall(model.ProviderIMDb, operation)
	}

	return providers.DoWithRetry(
		ctx,
		c.logger,
		model.ProviderIMDb,
		operation,
		c.retryCount,
		c.baseBackoff,
		c.sleep,
		func() (model.SelectedMatchResult, error) {
			query := url.Values{}
			query.Set("apikey", c.apiKey)
			query.Set("i", seriesID)
			query.Set("Season", strconv.Itoa(episode.SeasonNumber))
			query.Set("Episode", strconv.Itoa(episode.EpisodeNumber))
			query.Set("type", "episode")

			var resp omdbEpisodeResponse
			if err := c.get(ctx, query, &resp); err != nil {
				return model.SelectedMatchResult{}, err
			}
			if !resp.IsSuccess() {
				if isNotFoundAPIError(resp.Error) {
					// "Not found" for episode lookup is not fatal and must not fail the app.
					return model.SelectedMatchResult{}, nil
				}
				return model.SelectedMatchResult{}, fmt.Errorf("imdb api: %s", resp.Error)
			}

			year := parseYear(resp.Year)
			return model.SelectedMatchResult{
				Provider:      model.ProviderIMDb,
				Kind:          model.MediaKindEpisode,
				Title:         strings.TrimSpace(resp.Title),
				OriginalTitle: strings.TrimSpace(resp.Title),
				Year:          year,
				Episode: &model.EpisodeInfo{
					SeasonNumber:  episode.SeasonNumber,
					EpisodeNumber: episode.EpisodeNumber,
				},
				IDs: model.ProviderTags{
					IMDbID: strings.TrimSpace(resp.SeriesID),
				},
				EpisodeIDs: model.ProviderTags{
					IMDbID: strings.TrimSpace(resp.IMDbID),
				},
				ProviderReference: strings.TrimSpace(resp.IMDbID),
			}, nil
		},
	)
}

func (c *Client) search(ctx context.Context, operation string, queryTitle string, kind string) ([]model.SelectedMatchResult, error) {
	if c.logger != nil {
		c.logger.LogProviderCall(model.ProviderIMDb, operation)
	}

	return providers.DoWithRetry(
		ctx,
		c.logger,
		model.ProviderIMDb,
		operation,
		c.retryCount,
		c.baseBackoff,
		c.sleep,
		func() ([]model.SelectedMatchResult, error) {
			query := url.Values{}
			query.Set("apikey", c.apiKey)
			query.Set("s", queryTitle)
			query.Set("type", kind)

			var resp omdbSearchResponse
			if err := c.get(ctx, query, &resp); err != nil {
				return nil, err
			}
			if !resp.IsSuccess() {
				if isNonFatalSearchAPIError(resp.Error) {
					return []model.SelectedMatchResult{}, nil
				}
				return nil, fmt.Errorf("imdb api: %s", resp.Error)
			}

			results := make([]model.SelectedMatchResult, 0, len(resp.Search))
			targetKind := model.MediaKindMovie
			if kind == "series" {
				targetKind = model.MediaKindSeries
			}
			for _, it := range resp.Search {
				title := strings.TrimSpace(it.Title)
				if title == "" {
					continue
				}
				results = append(results, model.SelectedMatchResult{
					Provider:      model.ProviderIMDb,
					Kind:          targetKind,
					Title:         title,
					OriginalTitle: title,
					Year:          parseYear(it.Year),
					IDs: model.ProviderTags{
						IMDbID: strings.TrimSpace(it.IMDbID),
					},
					ProviderReference: strings.TrimSpace(it.IMDbID),
				})
			}
			return results, nil
		},
	)
}

func (c *Client) get(ctx context.Context, query url.Values, out any) error {
	if c.apiKey == "" {
		return fmt.Errorf("missing imdb api key")
	}
	u := c.baseURL + "/?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

func parseYear(s string) int {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return 0
	}
	for i := 0; i <= len(s)-4; i++ {
		part := s[i : i+4]
		if part[0] < '0' || part[0] > '9' {
			continue
		}
		year, err := strconv.Atoi(part)
		if err == nil && year >= 1900 && year <= 2099 {
			return year
		}
	}
	return 0
}

func isNotFoundAPIError(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(m, "not found")
}

func isTooManyResultsAPIError(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(m, "too many results")
}

func isNonFatalSearchAPIError(msg string) bool {
	return isNotFoundAPIError(msg) || isTooManyResultsAPIError(msg)
}

func parseOMDbType(v string) model.MediaKind {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "movie":
		return model.MediaKindMovie
	case "series":
		return model.MediaKindSeries
	case "episode":
		return model.MediaKindEpisode
	default:
		return model.MediaKindUnknown
	}
}
