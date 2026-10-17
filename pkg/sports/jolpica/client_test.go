package jolpica

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sports"
)

type obj = map[string]interface{}

func circuit(id, name, locality, country string) obj {
	return obj{"circuitId": id, "circuitName": name, "Location": obj{"locality": locality, "country": country}}
}

func sess(date, clock string) obj {
	ret := obj{"date": date}
	if clock != "" {
		ret["time"] = clock
	}
	return ret
}

func driver(given, family, code string) obj {
	return obj{"givenName": given, "familyName": family, "code": code}
}

// races mirror the shapes returned by api.jolpi.ca for these seasons.
var testRaces = map[int][]obj{
	2024: {
		{
			"season": "2024", "round": "1", "raceName": "Bahrain Grand Prix",
			"Circuit": circuit("bahrain", "Bahrain International Circuit", "Sakhir", "Bahrain"),
			"date":    "2024-03-02", "time": "15:00:00Z",
			"FirstPractice":  sess("2024-02-29", "11:30:00Z"),
			"SecondPractice": sess("2024-02-29", "15:00:00Z"),
			"ThirdPractice":  sess("2024-03-01", "12:30:00Z"),
			"Qualifying":     sess("2024-03-01", "16:00:00Z"),
		},
		{
			"season": "2024", "round": "21", "raceName": "São Paulo Grand Prix",
			"Circuit": circuit("interlagos", "Autódromo José Carlos Pace", "São Paulo", "Brazil"),
			"date":    "2024-11-03", "time": "17:00:00Z",
			"FirstPractice":    sess("2024-11-01", "14:30:00Z"),
			"Qualifying":       sess("2024-11-02", "18:00:00Z"),
			"Sprint":           sess("2024-11-02", "14:00:00Z"),
			"SprintQualifying": sess("2024-11-01", "18:30:00Z"),
		},
	},
	2023: {
		{
			// recorded Ergast-style response listing the shootout as SecondPractice
			"season": "2023", "round": "4", "raceName": "Azerbaijan Grand Prix",
			"Circuit": circuit("baku", "Baku City Circuit", "Baku", "Azerbaijan"),
			"date":    "2023-04-30", "time": "11:00:00Z",
			"FirstPractice":  sess("2023-04-28", "09:30:00Z"),
			"Qualifying":     sess("2023-04-28", "13:00:00Z"),
			"SecondPractice": sess("2023-04-29", "08:30:00Z"),
			"Sprint":         sess("2023-04-29", "13:30:00Z"),
		},
		{
			"season": "2023", "round": "10", "raceName": "Austrian Grand Prix",
			"Circuit": circuit("red_bull_ring", "Red Bull Ring", "Spielberg", "Austria"),
			"date":    "2023-07-02", "time": "13:00:00Z",
			"FirstPractice":  sess("2023-06-30", "11:30:00Z"),
			"Qualifying":     sess("2023-06-30", "15:00:00Z"),
			"SprintShootout": sess("2023-07-01", "10:00:00Z"),
			"Sprint":         sess("2023-07-01", "14:30:00Z"),
		},
	},
	2022: {
		{
			// 2022 sprint weekends had a real second practice on Saturday
			"season": "2022", "round": "4", "raceName": "Emilia Romagna Grand Prix",
			"Circuit": circuit("imola", "Autodromo Enzo e Dino Ferrari", "Imola", "Italy"),
			"date":    "2022-04-24", "time": "13:00:00Z",
			"FirstPractice":  sess("2022-04-22", "11:30:00Z"),
			"Qualifying":     sess("2022-04-22", "15:00:00Z"),
			"SecondPractice": sess("2022-04-23", "10:30:00Z"),
			"Sprint":         sess("2022-04-23", "14:30:00Z"),
		},
	},
	1998: {
		{
			"season": "1998", "round": "16", "raceName": "Japanese Grand Prix",
			"Circuit": circuit("suzuka", "Suzuka Circuit", "Suzuka", "Japan"),
			"date":    "1998-11-01",
		},
	},
}

var testResults = map[string]map[int][]obj{
	"results": {
		2024: {{"season": "2024", "round": "1", "Results": []obj{{"position": "1", "Driver": driver("Max", "Verstappen", "VER")}}},
			{"season": "2024", "round": "21", "Results": []obj{{"position": "1", "Driver": driver("Max", "Verstappen", "VER")}}}},
		1998: {{"season": "1998", "round": "16", "Results": []obj{{"position": "1", "Driver": driver("Mika", "Häkkinen", "HAK")}}}},
	},
	"qualifying": {
		// paged one race per page to exercise pagination
		2024: {{"season": "2024", "round": "1", "QualifyingResults": []obj{{"position": "1", "Driver": driver("Max", "Verstappen", "VER")}}},
			{"season": "2024", "round": "21", "QualifyingResults": []obj{{"position": "1", "Driver": driver("Lando", "Norris", "NOR")}}}},
	},
	"sprint": {
		2024: {{"season": "2024", "round": "21", "SprintResults": []obj{
			{"position": "2", "Driver": driver("Oscar", "Piastri", "PIA")},
			{"position": "1", "Driver": driver("Lando", "Norris", "NOR")},
		}}},
	},
}

type fakeAPI struct {
	mu        sync.Mutex
	paths     []string
	userAgent string
	// throttle answers this many requests with 429 before serving
	throttle int
}

func (f *fakeAPI) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.Path)
		f.userAgent = r.Header.Get("User-Agent")
		throttled := f.throttle > 0
		if throttled {
			f.throttle--
		}
		f.mu.Unlock()

		if throttled {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		assert.Equal(t, pageSize, limit)

		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/ergast/f1"), "/"), "/")
		writeRaces := func(all []obj, pageLimit int) {
			end := min(offset+pageLimit, len(all))
			page := []obj{}
			if offset < len(all) {
				page = all[offset:end]
			}
			_ = json.NewEncoder(w).Encode(obj{"MRData": obj{
				"limit": strconv.Itoa(pageLimit), "offset": strconv.Itoa(offset), "total": strconv.Itoa(len(all)),
				"RaceTable": obj{"Races": page},
			}})
		}

		switch {
		case len(parts) == 1 && parts[0] == "seasons.json":
			_ = json.NewEncoder(w).Encode(obj{"MRData": obj{
				"limit": "100", "offset": "0", "total": "3",
				"SeasonTable": obj{"Seasons": []obj{{"season": "2024"}, {"season": "1998"}, {"season": "2023"}}},
			}})
		case len(parts) == 2 && parts[1] == "races.json":
			year, _ := strconv.Atoi(parts[0])
			writeRaces(testRaces[year], 100)
		case len(parts) == 3 && parts[2] == "1.json":
			year, _ := strconv.Atoi(parts[0])
			pageLimit := 100
			if parts[1] == "qualifying" {
				pageLimit = 1
			}
			writeRaces(testResults[parts[1]][year], pageLimit)
		default:
			http.NotFound(w, r)
		}
	})
}

func newTestClient(t *testing.T, f *fakeAPI) (*Client, *[]time.Duration) {
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL+"/ergast/f1/", "SportsMediaCollection/test")
	c.MinInterval = 0
	var waits []time.Duration
	c.sleep = func(ctx context.Context, d time.Duration) error {
		if d > time.Millisecond {
			waits = append(waits, d)
		}
		return ctx.Err()
	}
	return c, &waits
}

func sessionsByType(e sports.CalendarEvent) map[models.SessionType]sports.CalendarSession {
	ret := map[models.SessionType]sports.CalendarSession{}
	for _, s := range e.Sessions {
		ret[s.Type] = s
	}
	return ret
}

func TestSeason2024(t *testing.T) {
	f := &fakeAPI{}
	c, _ := newTestClient(t, f)

	events, err := c.Season(context.Background(), 2024)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "SportsMediaCollection/test", f.userAgent)

	bahrain := events[0]
	assert.Equal(t, 1, bahrain.Round)
	assert.Equal(t, "Bahrain Grand Prix", bahrain.Name)
	assert.Equal(t, "2024/bahrain/bahrain_grand_prix", bahrain.ExternalID)
	assert.Equal(t, "Bahrain International Circuit", bahrain.Circuit)
	assert.Equal(t, "Sakhir", bahrain.Location)
	assert.Equal(t, "2024-02-29", bahrain.StartDate, "starts with first practice")
	assert.Equal(t, "2024-03-02", bahrain.EndDate)
	bs := sessionsByType(bahrain)
	assert.Len(t, bs, 5)
	assert.Equal(t, time.Date(2024, 3, 2, 15, 0, 0, 0, time.UTC), *bs[models.SessionTypeRace].Start)
	assert.Equal(t, "Max Verstappen", bs[models.SessionTypeRace].Winner)
	assert.Equal(t, "Max Verstappen", bs[models.SessionTypeQualifying].Winner)

	brazil := events[1]
	assert.Equal(t, "2024/interlagos/sao_paulo_grand_prix", brazil.ExternalID)
	ss := sessionsByType(brazil)
	assert.ElementsMatch(t, []models.SessionType{
		models.SessionTypePractice1, models.SessionTypeSprintQualifying, models.SessionTypeSprint,
		models.SessionTypeQualifying, models.SessionTypeRace,
	}, keys(ss))
	assert.Equal(t, "", ss[models.SessionTypeSprintQualifying].Name)
	assert.Equal(t, "Lando Norris", ss[models.SessionTypeQualifying].Winner, "pole from paged qualifying results")
	assert.Equal(t, "Lando Norris", ss[models.SessionTypeSprint].Winner, "the P1 entry, not the first listed")
	assert.Equal(t, "Max Verstappen", ss[models.SessionTypeRace].Winner)

	assert.Contains(t, f.paths, "/ergast/f1/2024/sprint/1.json")
}

func keys(m map[models.SessionType]sports.CalendarSession) []models.SessionType {
	var ret []models.SessionType
	for k := range m {
		ret = append(ret, k)
	}
	return ret
}

func TestSprintQualifyingNamesAcrossEras(t *testing.T) {
	c, _ := newTestClient(t, &fakeAPI{})

	events, err := c.Season(context.Background(), 2023)
	require.NoError(t, err)
	require.Len(t, events, 2)

	baku := sessionsByType(events[0])
	assert.NotContains(t, baku, models.SessionTypePractice2, "legacy SecondPractice is the shootout")
	require.Contains(t, baku, models.SessionTypeSprintQualifying)
	assert.Equal(t, "Sprint Shootout", baku[models.SessionTypeSprintQualifying].Name)
	assert.Equal(t, "2023-04-29", baku[models.SessionTypeSprintQualifying].Date)

	austria := sessionsByType(events[1])
	assert.Equal(t, "Sprint Shootout", austria[models.SessionTypeSprintQualifying].Name)

	events, err = c.Season(context.Background(), 2022)
	require.NoError(t, err)
	imola := sessionsByType(events[0])
	assert.Contains(t, imola, models.SessionTypePractice2, "2022 second practice was a real session")
	assert.NotContains(t, imola, models.SessionTypeSprintQualifying)
}

func TestHistoricSeasonWithoutTimes(t *testing.T) {
	f := &fakeAPI{}
	c, _ := newTestClient(t, f)

	events, err := c.Season(context.Background(), 1998)
	require.NoError(t, err)
	require.Len(t, events, 1)

	e := events[0]
	assert.Equal(t, "1998-11-01", e.StartDate)
	assert.Equal(t, "1998-11-01", e.EndDate)
	require.Len(t, e.Sessions, 1)
	assert.Equal(t, models.SessionTypeRace, e.Sessions[0].Type)
	assert.Nil(t, e.Sessions[0].Start, "no time known")
	assert.Equal(t, "1998-11-01", e.Sessions[0].Date)
	assert.Equal(t, "Mika Häkkinen", e.Sessions[0].Winner)

	assert.NotContains(t, f.paths, "/ergast/f1/1998/sprint/1.json", "no sprint request without sprints")
}

func TestSeasonsSortedAndEmptySeason(t *testing.T) {
	c, _ := newTestClient(t, &fakeAPI{})

	seasons, err := c.Seasons(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int{1998, 2023, 2024}, seasons)

	events, err := c.Season(context.Background(), 1949)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestThrottlingIsRetried(t *testing.T) {
	f := &fakeAPI{throttle: 2}
	c, waits := newTestClient(t, f)

	seasons, err := c.Seasons(context.Background())
	require.NoError(t, err)
	assert.Len(t, seasons, 3)
	assert.Equal(t, []time.Duration{7 * time.Second, 7 * time.Second}, *waits, "honours Retry-After")

	f.throttle = 10
	_, err = c.Seasons(context.Background())
	var se *StatusError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, http.StatusTooManyRequests, se.StatusCode)
}

func TestNotFoundIsNotRetried(t *testing.T) {
	f := &fakeAPI{}
	c, _ := newTestClient(t, f)

	_, err := c.races(context.Background(), "nope.json")
	var se *StatusError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, http.StatusNotFound, se.StatusCode)
	assert.Len(t, f.paths, 1)
}

func TestPacing(t *testing.T) {
	c := NewClient("http://unused", "x")
	c.MinInterval = time.Second
	var waits []time.Duration
	c.sleep = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}

	for i := 0; i < 3; i++ {
		require.NoError(t, c.wait(context.Background()))
	}
	require.Len(t, waits, 3)
	assert.LessOrEqual(t, waits[0], 10*time.Millisecond, "first request is immediate")
	assert.InDelta(t, float64(2*time.Second), float64(waits[2]), float64(100*time.Millisecond), "third waits two intervals")
}

func TestSlug(t *testing.T) {
	assert.Equal(t, "sao_paulo_grand_prix", slug("São Paulo Grand Prix"))
	assert.Equal(t, "70th_anniversary_grand_prix", slug("70th Anniversary Grand Prix"))
	assert.Equal(t, "grosser_preis_von_osterreich", slug("Großer Preis von Österreich"))
}

func TestRetryAfterParsing(t *testing.T) {
	h := http.Header{}
	assert.Equal(t, 10*time.Second, retryAfter(h, 1))
	assert.Equal(t, 20*time.Second, retryAfter(h, 2))
	h.Set("Retry-After", "3")
	assert.Equal(t, 3*time.Second, retryAfter(h, 1))
	h.Set("Retry-After", "100000")
	assert.Equal(t, maxRetryWait, retryAfter(h, 1))
}
