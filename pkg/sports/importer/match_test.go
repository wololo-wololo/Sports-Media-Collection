package importer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stashapp/stash/pkg/models"
)

type testCalendar struct {
	series   []*models.Series
	events   []*models.Event
	sessions []*models.EventSession
	byName   map[string]*models.Event
}

func (c *testCalendar) event(seriesID, season, round int, name, location, country, circuit, end string, sessions ...models.SessionType) {
	id := len(c.events) + 1
	r := round
	d, _ := models.ParseDate(end)
	e := &models.Event{ID: id, SeriesID: seriesID, Season: season, Round: &r, Name: name, Location: location, Country: country, Circuit: circuit, EndDate: &d}
	c.events = append(c.events, e)
	c.byName[name+" "+time.Date(season, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006")] = e
	for _, st := range sessions {
		c.sessions = append(c.sessions, &models.EventSession{ID: len(c.sessions) + 100, EventID: id, SessionType: st})
	}
}

func newTestMatcher(t *testing.T) (*Matcher, *testCalendar) {
	c := &testCalendar{byName: map[string]*models.Event{}}
	c.series = []*models.Series{{ID: 1, Name: "Formula 1", ShortName: "F1"}, {ID: 2, Name: "WEC"}}

	fp := []models.SessionType{models.SessionTypePractice1, models.SessionTypePractice2, models.SessionTypePractice3, models.SessionTypeQualifying, models.SessionTypeRace}
	sprint := []models.SessionType{models.SessionTypePractice1, models.SessionTypeSprintQualifying, models.SessionTypeSprint, models.SessionTypeQualifying, models.SessionTypeRace}

	c.event(1, 2024, 1, "Bahrain Grand Prix", "Sakhir", "Bahrain", "Bahrain International Circuit", "2024-03-02", fp...)
	c.event(1, 2024, 21, "São Paulo Grand Prix", "São Paulo", "Brazil", "Autódromo José Carlos Pace", "2024-11-03", sprint...)
	c.event(1, 2024, 22, "Las Vegas Grand Prix", "Las Vegas", "USA", "Las Vegas Strip Street Circuit", "2024-11-23", fp...)
	c.event(1, 2023, 20, "São Paulo Grand Prix", "São Paulo", "Brazil", "Autódromo José Carlos Pace", "2023-11-05", sprint...)
	c.event(1, 2022, 21, "Brazilian Grand Prix", "São Paulo", "Brazil", "Autódromo José Carlos Pace", "2022-11-13", sprint...)
	c.event(1, 2019, 20, "Brazilian Grand Prix", "São Paulo", "Brazil", "Autódromo José Carlos Pace", "2019-11-17", fp...)
	c.event(1, 1998, 16, "Japanese Grand Prix", "Suzuka", "Japan", "Suzuka Circuit", "1998-11-01", models.SessionTypeRace)
	c.event(1, 1999, 14, "European Grand Prix", "Nürburg", "Germany", "Nürburgring", "1999-09-26", models.SessionTypeRace)
	c.event(2, 2023, 4, "24 Hours of Le Mans", "Le Mans", "France", "Circuit de la Sarthe", "2023-06-11")

	broadcasters := []Broadcaster{{ID: 7, Name: "Sky Sports F1", Aliases: []string{"SkyF1"}}, {ID: 8, Name: "RTL"}}
	return NewMatcher(c.series, c.events, c.sessions, broadcasters, nil), c
}

func TestMatch(t *testing.T) {
	m, c := newTestMatcher(t)
	brazil24 := c.byName["São Paulo Grand Prix 2024"]
	vegas := c.byName["Las Vegas Grand Prix 2024"]

	t.Run("full scene name", func(t *testing.T) {
		s := m.Match(`D:\F1\2024\Formula1.2024.Round21.Sao.Paulo.Race.SkyF1.1080p50.mkv`, time.Time{})
		assert.Equal(t, ConfidenceHigh, s.Confidence, s.Notes)
		assert.Equal(t, brazil24.ID, s.Event.ID)
		require.NotNil(t, s.Session)
		assert.Equal(t, models.SessionTypeRace, s.Session.SessionType)
		assert.Equal(t, 7, *s.StudioID, "existing broadcaster matched")
		assert.Equal(t, "en", s.Language)
		assert.Equal(t, models.FeedTypeWorldFeed, *s.FeedType)
		assert.True(t, fieldGuessed(s, "language"), "language comes from the broadcaster")
		assert.False(t, fieldGuessed(s, "event"))
	})

	t.Run("event by name without round", func(t *testing.T) {
		s := m.Match(`/F1/F1.2024.Las.Vegas.Qualifying.SkyF1.mkv`, time.Time{})
		assert.Equal(t, ConfidenceHigh, s.Confidence, s.Notes)
		assert.Equal(t, vegas.ID, s.Event.ID)
		assert.Equal(t, models.SessionTypeQualifying, s.Session.SessionType)
	})

	t.Run("onboard without series", func(t *testing.T) {
		s := m.Match(`/captures/Onboard_PIA_Race_LasVegas2024.mp4`, time.Time{})
		assert.Equal(t, ConfidenceMedium, s.Confidence, "series guessed")
		assert.Equal(t, vegas.ID, s.Event.ID)
		assert.Equal(t, models.FeedTypeOnboard, *s.FeedType)
		assert.Equal(t, "PIA", s.FeedSubject)
		assert.Equal(t, "", s.Language, "onboards carry original audio")
	})

	t.Run("dvd folder", func(t *testing.T) {
		s := m.Match(`D:\Motorsport\DVD\F1 1999 Nürburgring RTL\VIDEO_TS\VTS_01_1.VOB`, time.Time{})
		assert.Equal(t, ConfidenceMedium, s.Confidence, "session not named")
		assert.Equal(t, "European Grand Prix", s.Event.Name)
		assert.Equal(t, models.SessionTypeRace, s.Session.SessionType)
		assert.True(t, fieldGuessed(s, "session"))
		assert.Equal(t, models.RecordingSourceDVD, *s.Source)
		assert.Equal(t, 8, *s.StudioID)
		assert.Equal(t, "de", s.Language)
	})

	t.Run("no year picks the nearest sprint weekend", func(t *testing.T) {
		fileDate := time.Date(2024, 11, 2, 20, 0, 0, 0, time.UTC)
		s := m.Match(`E:\Captures\gp_brasil_sprint.ts`, fileDate)
		assert.Equal(t, ConfidenceLow, s.Confidence)
		require.Len(t, s.Alternatives, 3, "only weekends with a sprint")
		assert.Equal(t, 2024, s.Alternatives[0].Season)
		assert.Equal(t, 2023, s.Alternatives[1].Season)
		assert.Equal(t, brazil24.ID, s.Event.ID)
		assert.Contains(t, s.Notes, "No year in the name; pick the right weekend")
		assert.True(t, fieldGuessed(s, "source"))
	})

	t.Run("german naming", func(t *testing.T) {
		s := m.Match(`/f1/Formel 1 - Großer Preis von Japan 1998 - Rennen - RTL.avi`, time.Time{})
		assert.Equal(t, ConfidenceHigh, s.Confidence, s.Notes)
		assert.Equal(t, "Japanese Grand Prix", s.Event.Name)
		assert.Equal(t, "de", s.Language)
	})

	t.Run("session missing from an old calendar", func(t *testing.T) {
		s := m.Match(`/f1/F1 1998 Suzuka Qualifying.mkv`, time.Time{})
		assert.Equal(t, ConfidenceMedium, s.Confidence)
		assert.Equal(t, "Japanese Grand Prix", s.Event.Name, "event still found")
		assert.Nil(t, s.Session)
		assert.Contains(t, s.Notes, "The calendar has no qualifying session for this event")
	})

	t.Run("round contradicts the name", func(t *testing.T) {
		s := m.Match(`/f1/F1 2024 R21 Las Vegas Race.mkv`, time.Time{})
		assert.Equal(t, ConfidenceMedium, s.Confidence)
		assert.Equal(t, brazil24.ID, s.Event.ID)
		assert.Contains(t, s.Notes, "Round 21 is São Paulo Grand Prix, which the name does not mention")
	})

	t.Run("other series", func(t *testing.T) {
		s := m.Match(`D:\Motorsport\WEC\2023 Le Mans part 4.mkv`, time.Time{})
		assert.Equal(t, "24 Hours of Le Mans", s.Event.Name)
		assert.Equal(t, 4, s.Part)
		assert.Equal(t, ConfidenceMedium, s.Confidence, "the calendar has no sessions")
	})

	t.Run("ambiguous Sky resolved by language", func(t *testing.T) {
		s := m.Match(`/f1/F1 2024 Bahrain Race Sky DE.mkv`, time.Time{})
		assert.Equal(t, ConfidenceHigh, s.Confidence, s.Notes)
		assert.Nil(t, s.StudioID, "Sky Deutschland is not a studio yet")
		assert.Equal(t, "Sky Deutschland", s.BroadcasterName)
		assert.Equal(t, "de", s.Language)
	})

	t.Run("ambiguous Sky without language", func(t *testing.T) {
		s := m.Match(`/f1/F1 2024 Bahrain Race Sky.mkv`, time.Time{})
		assert.Equal(t, ConfidenceMedium, s.Confidence)
		assert.Equal(t, 7, *s.StudioID)
		assert.True(t, fieldGuessed(s, "broadcaster"))
	})

	t.Run("year from the file date is only a guess", func(t *testing.T) {
		s := m.Match(`/f1/Bahrain Race.mkv`, time.Time{})
		assert.Equal(t, ConfidenceMedium, s.Confidence)
		assert.Equal(t, "Bahrain Grand Prix", s.Event.Name)
		assert.True(t, fieldGuessed(s, "event"))
	})

	t.Run("series not in the library", func(t *testing.T) {
		s := m.Match(`/motogp/MotoGP 2015 Valencia Race.mp4`, time.Time{})
		assert.Equal(t, ConfidenceNone, s.Confidence)
		assert.Nil(t, s.Event)
		assert.Contains(t, s.Notes, "MotoGP is not in the library yet")
	})

	t.Run("event not in the calendar", func(t *testing.T) {
		s := m.Match(`/f1/German GP 1998 RTL.mkv`, time.Time{})
		assert.Equal(t, ConfidenceNone, s.Confidence)
		assert.Equal(t, 8, *s.StudioID, "fields are still suggested")
	})

	t.Run("nothing recognisable", func(t *testing.T) {
		s := m.Match(`/videos/IMG_2041.mov`, time.Time{})
		assert.Equal(t, ConfidenceNone, s.Confidence)
		assert.Empty(t, s.Fields)
	})
}

func fieldGuessed(s Suggestion, name string) bool {
	for _, f := range s.Fields {
		if f.Name == name {
			return f.Guessed
		}
	}
	return false
}

func TestConfidenceAtLeast(t *testing.T) {
	assert.True(t, ConfidenceHigh.AtLeast(ConfidenceMedium))
	assert.True(t, ConfidenceMedium.AtLeast(ConfidenceMedium))
	assert.False(t, ConfidenceLow.AtLeast(ConfidenceMedium))
	assert.False(t, ConfidenceNone.AtLeast(ConfidenceLow))
}

func TestMatchEvent(t *testing.T) {
	m, c := newTestMatcher(t)
	brazil23 := c.byName["São Paulo Grand Prix 2023"]

	s, err := m.MatchEvent(`E:\Captures\gp_brasil_sprint.ts`, time.Time{}, brazil23.ID)
	require.NoError(t, err)
	assert.Equal(t, brazil23.ID, s.Event.ID)
	assert.Equal(t, models.SessionTypeSprint, s.Session.SessionType, "session of the chosen event")
	assert.Empty(t, s.Alternatives)
	assert.Equal(t, ConfidenceHigh, s.Confidence)

	_, err = m.MatchEvent(`/x.mkv`, time.Time{}, 9999)
	assert.Error(t, err)
}
