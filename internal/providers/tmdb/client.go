package tmdb

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

const defaultBaseURL = "https://api.themoviedb.org"

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
	if c.logger != nil {
		c.logger.LogProviderCall(model.ProviderTMDb, "search_movie")
	}

	return providers.DoWithRetry(
		ctx,
		c.logger,
		model.ProviderTMDb,
		"search_movie",
		c.retryCount,
		c.baseBackoff,
		c.sleep,
		func() ([]model.SelectedMatchResult, error) {
			q := url.Values{}
			q.Set("api_key", c.apiKey)
			q.Set("query", candidate.QueryTitle)
			if candidate.QueryYear > 0 {
				q.Set("year", strconv.Itoa(candidate.QueryYear))
			}
			var resp searchMovieResponse
			if err := c.getJSON(ctx, "/3/search/movie", q, &resp); err != nil {
				return nil, err
			}
			return mapMovieResults(resp.Results), nil
		},
	)
}

func (c *Client) SearchSeries(ctx context.Context, candidate model.ProviderSearchCandidate) ([]model.SelectedMatchResult, error) {
	if c.logger != nil {
		c.logger.LogProviderCall(model.ProviderTMDb, "search_series")
	}

	return providers.DoWithRetry(
		ctx,
		c.logger,
		model.ProviderTMDb,
		"search_series",
		c.retryCount,
		c.baseBackoff,
		c.sleep,
		func() ([]model.SelectedMatchResult, error) {
			q := url.Values{}
			q.Set("api_key", c.apiKey)
			q.Set("query", candidate.QueryTitle)
			if candidate.QueryYear > 0 {
				q.Set("first_air_date_year", strconv.Itoa(candidate.QueryYear))
			}
			var resp searchTVResponse
			if err := c.getJSON(ctx, "/3/search/tv", q, &resp); err != nil {
				return nil, err
			}
			return mapSeriesResults(resp.Results), nil
		},
	)
}

func (c *Client) LookupEpisode(ctx context.Context, series model.SelectedMatchResult, episode model.EpisodeInfo) (model.SelectedMatchResult, error) {
	if c.logger != nil {
		c.logger.LogProviderCall(model.ProviderTMDb, "lookup_episode")
	}

	seriesID := strings.TrimSpace(series.ProviderReference)
	if seriesID == "" {
		seriesID = strings.TrimSpace(series.IDs.TMDbID)
	}
	if seriesID == "" {
		return model.SelectedMatchResult{}, fmt.Errorf("tmdb episode lookup requires series tmdb id")
	}

	return providers.DoWithRetry(
		ctx,
		c.logger,
		model.ProviderTMDb,
		"lookup_episode",
		c.retryCount,
		c.baseBackoff,
		c.sleep,
		func() (model.SelectedMatchResult, error) {
			q := url.Values{}
			q.Set("api_key", c.apiKey)
			q.Set("append_to_response", "external_ids")

			path := fmt.Sprintf("/3/tv/%s/season/%d/episode/%d", seriesID, episode.SeasonNumber, episode.EpisodeNumber)
			var resp episodeDetailResponse
			if err := c.getJSON(ctx, path, q, &resp); err != nil {
				return model.SelectedMatchResult{}, err
			}

			epTMDbID := ""
			if resp.ID > 0 {
				epTMDbID = strconv.Itoa(resp.ID)
			}
			return model.SelectedMatchResult{
				Provider:      model.ProviderTMDb,
				Kind:          model.MediaKindEpisode,
				Title:         strings.TrimSpace(resp.Name),
				OriginalTitle: strings.TrimSpace(resp.Name),
				Year:          yearFromDate(resp.AirDate),
				Episode: &model.EpisodeInfo{
					SeasonNumber:  episode.SeasonNumber,
					EpisodeNumber: episode.EpisodeNumber,
				},
				IDs: model.ProviderTags{
					TMDbID: seriesID,
				},
				EpisodeIDs: model.ProviderTags{
					IMDbID: strings.TrimSpace(resp.ExternalIDs.IMDbID),
					TMDbID: epTMDbID,
				},
				ProviderReference: epTMDbID,
			}, nil
		},
	)
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	if c.apiKey == "" {
		return fmt.Errorf("missing tmdb api key")
	}
	u := c.baseURL + path
	if query != nil && len(query) > 0 {
		u += "?" + query.Encode()
	}
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
