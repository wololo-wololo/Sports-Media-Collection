// Package jolpica reads Formula 1 calendars and results from the Jolpica-F1
// API (https://github.com/jolpica/jolpica-f1), the successor of Ergast.
package jolpica

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sports"
)

const (
	// DefaultBaseURL is the Ergast-compatible Formula 1 endpoint.
	DefaultBaseURL = "https://api.jolpi.ca/ergast/f1"
	// DataSource is the series data_source value for this source.
	DataSource = "jolpica-f1"

	pageSize = 100

	// Unauthenticated access allows 4 requests per second in bursts and 500
	// per hour. Pacing at 3 per second keeps a full-history sync (about 300
	// requests) under both.
	defaultMinInterval = 340 * time.Millisecond
	defaultMaxRetries  = 3
	defaultRetryWait   = 10 * time.Second
	maxRetryWait       = 2 * time.Minute
)

// Client is a Jolpica-F1 API client. It is safe for concurrent use and paces
// its own requests.
type Client struct {
	BaseURL    string
	UserAgent  string
	HTTPClient *http.Client

	// MinInterval is the minimum time between two requests.
	MinInterval time.Duration
	// MaxRetries is how often a throttled (HTTP 429) or failed request is
	// retried.
	MaxRetries int

	// sleep waits for d or until ctx is done; replaceable in tests.
	sleep func(ctx context.Context, d time.Duration) error

	mu   sync.Mutex
	last time.Time
}

// NewClient returns a client for baseURL (DefaultBaseURL when empty).
// The API requires an identifying User-Agent such as "AppName/1.2.3".
func NewClient(baseURL string, userAgent string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL:     strings.TrimRight(baseURL, "/"),
		UserAgent:   userAgent,
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
		MinInterval: defaultMinInterval,
		MaxRetries:  defaultMaxRetries,
		sleep:       sleepCtx,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// ---------------------------------------------------------------------------
// response types

type mrData struct {
	MRData struct {
		Limit       string `json:"limit"`
		Offset      string `json:"offset"`
		Total       string `json:"total"`
		SeasonTable *struct {
			Seasons []struct {
				Season string `json:"season"`
			} `json:"Seasons"`
		} `json:"SeasonTable"`
		RaceTable *struct {
			Races []apiRace `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

type apiSession struct {
	Date string `json:"date"`
	Time string `json:"time"`
}

type apiDriver struct {
	Code       string `json:"code"`
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
}

func (d apiDriver) fullName() string {
	return strings.TrimSpace(d.GivenName + " " + d.FamilyName)
}

type apiResult struct {
	Position string    `json:"position"`
	Driver   apiDriver `json:"Driver"`
}

type apiRace struct {
	Season   string `json:"season"`
	Round    string `json:"round"`
	RaceName string `json:"raceName"`
	Circuit  struct {
		CircuitID   string `json:"circuitId"`
		CircuitName string `json:"circuitName"`
		Location    struct {
			Locality string `json:"locality"`
			Country  string `json:"country"`
		} `json:"Location"`
	} `json:"Circuit"`
	Date string `json:"date"`
	Time string `json:"time"`

	FirstPractice    *apiSession `json:"FirstPractice"`
	SecondPractice   *apiSession `json:"SecondPractice"`
	ThirdPractice    *apiSession `json:"ThirdPractice"`
	Qualifying       *apiSession `json:"Qualifying"`
	Sprint           *apiSession `json:"Sprint"`
	SprintQualifying *apiSession `json:"SprintQualifying"`
	SprintShootout   *apiSession `json:"SprintShootout"`

	Results           []apiResult `json:"Results"`
	QualifyingResults []apiResult `json:"QualifyingResults"`
	SprintResults     []apiResult `json:"SprintResults"`
}

// ---------------------------------------------------------------------------
// requests

// StatusError is returned for unexpected HTTP responses.
type StatusError struct {
	URL        string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("jolpica-f1: %s returned HTTP %d", e.URL, e.StatusCode)
}

func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	next := c.last.Add(c.MinInterval)
	now := time.Now()
	if next.Before(now) {
		next = now
	}
	c.last = next
	c.mu.Unlock()

	return c.sleep(ctx, time.Until(next))
}

func retryAfter(h http.Header, attempt int) time.Duration {
	if v := h.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
			return min(time.Duration(secs)*time.Second, maxRetryWait)
		}
		if t, err := http.ParseTime(v); err == nil {
			return min(max(time.Until(t), 0), maxRetryWait)
		}
	}
	return min(defaultRetryWait*time.Duration(attempt), maxRetryWait)
}

func (c *Client) get(ctx context.Context, path string, offset int) (*mrData, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(pageSize))
	q.Set("offset", strconv.Itoa(offset))
	u := c.BaseURL + "/" + strings.TrimLeft(path, "/") + "?" + q.Encode()

	for attempt := 1; ; attempt++ {
		if err := c.wait(ctx); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		if c.UserAgent != "" {
			req.Header.Set("User-Agent", c.UserAgent)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			if ctx.Err() != nil || attempt > c.MaxRetries {
				return nil, fmt.Errorf("jolpica-f1: requesting %s: %w", u, err)
			}
			logger.Warnf("jolpica-f1: request failed, retrying: %v", err)
			if err := c.sleep(ctx, defaultRetryWait*time.Duration(attempt)); err != nil {
				return nil, err
			}
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			wait := retryAfter(resp.Header, attempt)
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if attempt > c.MaxRetries {
				return nil, &StatusError{URL: u, StatusCode: resp.StatusCode}
			}
			logger.Infof("jolpica-f1: HTTP %d, waiting %s before retrying", resp.StatusCode, wait)
			if err := c.sleep(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil, &StatusError{URL: u, StatusCode: resp.StatusCode}
		}

		var ret mrData
		err = json.NewDecoder(resp.Body).Decode(&ret)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("jolpica-f1: decoding %s: %w", u, err)
		}
		return &ret, nil
	}
}

// getAll pages through path and calls fn for each page.
func (c *Client) getAll(ctx context.Context, path string, fn func(*mrData)) error {
	for offset := 0; ; {
		page, err := c.get(ctx, path, offset)
		if err != nil {
			return err
		}
		fn(page)

		total, _ := strconv.Atoi(page.MRData.Total)
		limit, _ := strconv.Atoi(page.MRData.Limit)
		if limit <= 0 {
			limit = pageSize
		}
		offset += limit
		if offset >= total {
			return nil
		}
	}
}

func (c *Client) races(ctx context.Context, path string) ([]apiRace, error) {
	var ret []apiRace
	err := c.getAll(ctx, path, func(p *mrData) {
		if p.MRData.RaceTable != nil {
			ret = append(ret, p.MRData.RaceTable.Races...)
		}
	})
	return ret, err
}

// ---------------------------------------------------------------------------
// public API

// Seasons returns all championship years, oldest first.
func (c *Client) Seasons(ctx context.Context) ([]int, error) {
	var ret []int
	err := c.getAll(ctx, "seasons.json", func(p *mrData) {
		if p.MRData.SeasonTable == nil {
			return
		}
		for _, s := range p.MRData.SeasonTable.Seasons {
			if y, err := strconv.Atoi(s.Season); err == nil {
				ret = append(ret, y)
			}
		}
	})
	if err != nil {
		return nil, err
	}
	sort.Ints(ret)
	return ret, nil
}

// winners maps round to the classified winner of a results listing.
func (c *Client) winners(ctx context.Context, path string, pick func(apiRace) []apiResult) (map[int]string, error) {
	races, err := c.races(ctx, path)
	if err != nil {
		return nil, err
	}

	ret := make(map[int]string)
	for _, r := range races {
		round, err := strconv.Atoi(r.Round)
		if err != nil {
			continue
		}
		for _, res := range pick(r) {
			if res.Position == "1" {
				ret[round] = res.Driver.fullName()
			}
		}
	}
	return ret, nil
}

// Season returns the calendar of a season with race winners, pole sitters
// and sprint winners where the results are known.
func (c *Client) Season(ctx context.Context, year int) ([]sports.CalendarEvent, error) {
	races, err := c.races(ctx, fmt.Sprintf("%d/races.json", year))
	if err != nil {
		return nil, err
	}
	if len(races) == 0 {
		return nil, nil
	}

	raceWinners, err := c.winners(ctx, fmt.Sprintf("%d/results/1.json", year), func(r apiRace) []apiResult { return r.Results })
	if err != nil {
		return nil, fmt.Errorf("race results: %w", err)
	}

	poles, err := c.winners(ctx, fmt.Sprintf("%d/qualifying/1.json", year), func(r apiRace) []apiResult { return r.QualifyingResults })
	if err != nil {
		return nil, fmt.Errorf("qualifying results: %w", err)
	}

	sprintWinners := map[int]string{}
	if hasSprint(races) {
		sprintWinners, err = c.winners(ctx, fmt.Sprintf("%d/sprint/1.json", year), func(r apiRace) []apiResult { return r.SprintResults })
		if err != nil {
			return nil, fmt.Errorf("sprint results: %w", err)
		}
	}

	ret := make([]sports.CalendarEvent, 0, len(races))
	for _, r := range races {
		e, err := convertRace(r)
		if err != nil {
			logger.Warnf("jolpica-f1: skipping race %q: %v", r.RaceName, err)
			continue
		}
		for i := range e.Sessions {
			s := &e.Sessions[i]
			switch s.Type {
			case models.SessionTypeRace:
				s.Winner = raceWinners[e.Round]
			case models.SessionTypeQualifying:
				s.Winner = poles[e.Round]
			case models.SessionTypeSprint:
				s.Winner = sprintWinners[e.Round]
			}
		}
		ret = append(ret, e)
	}
	return ret, nil
}

func hasSprint(races []apiRace) bool {
	for _, r := range races {
		if r.Sprint != nil {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// conversion

var slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

// slugLetters maps letters that have no decomposed accent-free form.
var slugLetters = strings.NewReplacer("ß", "ss", "æ", "ae", "œ", "oe", "ø", "o", "ł", "l", "đ", "d", "ı", "i")

// slug lower-cases s, strips accents ("São" -> "sao") and joins words
// with underscores.
func slug(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(slugLetters.Replace(strings.ToLower(s))) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return strings.Trim(slugInvalid.ReplaceAllString(b.String(), "_"), "_")
}

// externalID identifies a race by season, circuit and name, which unlike the
// round number survives calendar changes within a season.
func externalID(season int, circuitID string, raceName string) string {
	return fmt.Sprintf("%d/%s/%s", season, circuitID, slug(raceName))
}

func parseStart(date, clock string) *time.Time {
	if date == "" || clock == "" {
		return nil
	}
	if !strings.HasSuffix(clock, "Z") && !strings.ContainsAny(clock, "+-") {
		clock += "Z"
	}
	t, err := time.Parse(time.RFC3339, date+"T"+clock)
	if err != nil {
		return nil
	}
	t = t.UTC()
	return &t
}

func session(t models.SessionType, name string, s *apiSession) *sports.CalendarSession {
	if s == nil || s.Date == "" {
		return nil
	}
	return &sports.CalendarSession{Type: t, Name: name, Date: s.Date, Start: parseStart(s.Date, s.Time)}
}

func convertRace(r apiRace) (sports.CalendarEvent, error) {
	season, err := strconv.Atoi(r.Season)
	if err != nil {
		return sports.CalendarEvent{}, fmt.Errorf("invalid season %q", r.Season)
	}
	round, err := strconv.Atoi(r.Round)
	if err != nil {
		return sports.CalendarEvent{}, fmt.Errorf("invalid round %q", r.Round)
	}
	if r.Date == "" {
		return sports.CalendarEvent{}, errors.New("race has no date")
	}

	e := sports.CalendarEvent{
		Season:     season,
		Round:      round,
		Name:       r.RaceName,
		ExternalID: externalID(season, r.Circuit.CircuitID, r.RaceName),
		Circuit:    r.Circuit.CircuitName,
		Location:   r.Circuit.Location.Locality,
		Country:    r.Circuit.Location.Country,
		EndDate:    r.Date,
	}

	sprintQuali, sprintQualiName := r.SprintQualifying, ""
	if r.SprintShootout != nil {
		sprintQuali, sprintQualiName = r.SprintShootout, "Sprint Shootout"
	}

	secondPractice := r.SecondPractice
	// Ergast listed the 2023 Sprint Shootout and later Sprint Qualifying as
	// "SecondPractice". Before 2023 a sprint weekend's second practice was a
	// real practice session.
	if sprintQuali == nil && r.Sprint != nil && secondPractice != nil && r.ThirdPractice == nil && season >= 2023 {
		sprintQuali, secondPractice = secondPractice, nil
		if season == 2023 {
			sprintQualiName = "Sprint Shootout"
		}
	}

	candidates := []*sports.CalendarSession{
		session(models.SessionTypePractice1, "", r.FirstPractice),
		session(models.SessionTypePractice2, "", secondPractice),
		session(models.SessionTypePractice3, "", r.ThirdPractice),
		session(models.SessionTypeSprintQualifying, sprintQualiName, sprintQuali),
		session(models.SessionTypeSprint, "", r.Sprint),
		session(models.SessionTypeQualifying, "", r.Qualifying),
		session(models.SessionTypeRace, "", &apiSession{Date: r.Date, Time: r.Time}),
	}

	e.StartDate = r.Date
	for _, s := range candidates {
		if s == nil {
			continue
		}
		e.Sessions = append(e.Sessions, *s)
		if s.Date < e.StartDate {
			e.StartDate = s.Date
		}
	}

	return e, nil
}
