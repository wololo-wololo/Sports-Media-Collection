//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sports"
)

func syncRepo() sports.SyncRepository {
	return sports.SyncRepository{Series: db.Series, Event: db.Event, EventSession: db.EventSession}
}

func utc(y int, m time.Month, d, h, min int) *time.Time {
	t := time.Date(y, m, d, h, min, 0, 0, time.UTC)
	return &t
}

func calendar2024() []sports.CalendarEvent {
	return []sports.CalendarEvent{
		{
			Season: 2024, Round: 1, Name: "Bahrain Grand Prix", ExternalID: "2024/bahrain/bahrain_grand_prix",
			Circuit: "Bahrain International Circuit", Location: "Sakhir", Country: "Bahrain",
			StartDate: "2024-02-29", EndDate: "2024-03-02",
			Sessions: []sports.CalendarSession{
				{Type: models.SessionTypePractice1, Start: utc(2024, 2, 29, 11, 30)},
				{Type: models.SessionTypeQualifying, Start: utc(2024, 3, 1, 16, 0), Winner: "Max Verstappen"},
				{Type: models.SessionTypeRace, Start: utc(2024, 3, 2, 15, 0), Winner: "Max Verstappen"},
			},
		},
		{
			Season: 2024, Round: 21, Name: "São Paulo Grand Prix", ExternalID: "2024/interlagos/sao_paulo_grand_prix",
			Circuit: "Autódromo José Carlos Pace", Location: "São Paulo", Country: "Brazil",
			StartDate: "2024-11-01", EndDate: "2024-11-03",
			Sessions: []sports.CalendarSession{
				{Type: models.SessionTypeSprintQualifying, Start: utc(2024, 11, 1, 18, 30)},
				{Type: models.SessionTypeSprint, Start: utc(2024, 11, 2, 14, 0), Winner: "Lando Norris"},
				{Type: models.SessionTypeQualifying, Start: utc(2024, 11, 2, 18, 0), Winner: "Lando Norris"},
				{Type: models.SessionTypeRace, Start: utc(2024, 11, 3, 17, 0), Winner: "Max Verstappen"},
			},
		},
	}
}

func sessionOfType(t *testing.T, ctx context.Context, eventID int, st models.SessionType) *models.EventSession {
	t.Helper()
	sessions, err := db.EventSession.FindByEventID(ctx, eventID)
	require.NoError(t, err)
	for _, s := range sessions {
		if s.SessionType == st {
			return s
		}
	}
	t.Fatalf("no %s session for event %d", st, eventID)
	return nil
}

func TestEnsureSeries(t *testing.T) {
	runWithRollbackTxn(t, "creates then reuses", func(t *testing.T, ctx context.Context) {
		s, err := sports.EnsureSeries(ctx, syncRepo(), sports.Formula1)
		require.NoError(t, err)
		assert.Equal(t, "Formula 1", s.Name)
		assert.Equal(t, "jolpica-f1", s.DataSource)
		assert.Equal(t, "#E8473C", s.Color)

		again, err := sports.EnsureSeries(ctx, syncRepo(), sports.Formula1)
		require.NoError(t, err)
		assert.Equal(t, s.ID, again.ID)
	})

	runWithRollbackTxn(t, "links a hand-made series without changing it", func(t *testing.T, ctx context.Context) {
		mine := createSeries(t, ctx, "formula 1")
		p := models.NewSeriesPartial()
		p.Color = models.NewOptionalString("#123456")
		_, err := db.Series.UpdatePartial(ctx, mine.ID, p)
		require.NoError(t, err)

		s, err := sports.EnsureSeries(ctx, syncRepo(), sports.Formula1)
		require.NoError(t, err)
		assert.Equal(t, mine.ID, s.ID)
		assert.Equal(t, "jolpica-f1", s.DataSource)
		assert.Equal(t, "#123456", s.Color, "user's colour kept")
		assert.Equal(t, "formula 1", s.Name)
	})
}

func TestSyncSeason(t *testing.T) {
	runWithRollbackTxn(t, "creates, then is idempotent", func(t *testing.T, ctx context.Context) {
		f1, err := sports.EnsureSeries(ctx, syncRepo(), sports.Formula1)
		require.NoError(t, err)

		r, err := sports.SyncSeason(ctx, syncRepo(), f1.ID, 2024, calendar2024())
		require.NoError(t, err)
		assert.Equal(t, sports.SyncResult{EventsCreated: 2, SessionsCreated: 7}, r)

		season := 2024
		events, err := db.Event.Query(ctx, models.EventQueryOptions{SeriesID: &f1.ID, Season: &season})
		require.NoError(t, err)
		require.Len(t, events, 2)
		brazil := events[1]
		assert.Equal(t, 21, *brazil.Round)
		assert.Equal(t, "Autódromo José Carlos Pace", brazil.Circuit)
		assert.Equal(t, "2024-11-01", brazil.StartDate.String())
		assert.Equal(t, "2024-11-03", brazil.EndDate.String())
		assert.Equal(t, "Lando Norris", sessionOfType(t, ctx, brazil.ID, models.SessionTypeQualifying).Winner)

		r, err = sports.SyncSeason(ctx, syncRepo(), f1.ID, 2024, calendar2024())
		require.NoError(t, err)
		assert.Equal(t, sports.SyncResult{}, r, "second sync changes nothing")
	})

	runWithRollbackTxn(t, "keeps user corrections, follows corrected results", func(t *testing.T, ctx context.Context) {
		f1, err := sports.EnsureSeries(ctx, syncRepo(), sports.Formula1)
		require.NoError(t, err)
		_, err = sports.SyncSeason(ctx, syncRepo(), f1.ID, 2024, calendar2024())
		require.NoError(t, err)

		e, err := db.Event.FindByExternalID(ctx, f1.ID, "2024/interlagos/sao_paulo_grand_prix")
		require.NoError(t, err)

		// the user renames the event and records that rain moved qualifying
		ep := models.NewEventPartial()
		ep.Name = models.NewOptionalString("Großer Preis von Brasilien")
		_, err = db.Event.UpdatePartial(ctx, e.ID, ep)
		require.NoError(t, err)
		quali := sessionOfType(t, ctx, e.ID, models.SessionTypeQualifying)
		sp := models.NewEventSessionPartial()
		sp.StartTime = models.NewOptionalTime(*utc(2024, 11, 3, 10, 30))
		_, err = db.EventSession.UpdatePartial(ctx, quali.ID, sp)
		require.NoError(t, err)

		// the source later corrects the race winner
		cal := calendar2024()
		cal[1].Sessions[3].Winner = "Esteban Ocon"
		r, err := sports.SyncSeason(ctx, syncRepo(), f1.ID, 2024, cal)
		require.NoError(t, err)
		assert.Equal(t, sports.SyncResult{SessionsUpdated: 1}, r)

		e, err = db.Event.Find(ctx, e.ID)
		require.NoError(t, err)
		assert.Equal(t, "Großer Preis von Brasilien", e.Name)
		assert.True(t, sessionOfType(t, ctx, e.ID, models.SessionTypeQualifying).StartTime.Equal(*utc(2024, 11, 3, 10, 30)))
		assert.Equal(t, "Esteban Ocon", sessionOfType(t, ctx, e.ID, models.SessionTypeRace).Winner)
	})

	runWithRollbackTxn(t, "adopts hand-made events and keeps their recordings", func(t *testing.T, ctx context.Context) {
		f1, err := sports.EnsureSeries(ctx, syncRepo(), sports.Formula1)
		require.NoError(t, err)

		mine := createEvent(t, ctx, f1.ID, 2024, intPtr(21), "Brazil 2024")
		myRace := createSession(t, ctx, mine.ID, models.SessionTypeRace, nil)
		rec := createRecording(t, ctx, "Sky race", func(s *models.Scene) {
			s.EventID = &mine.ID
			s.SessionID = &myRace.ID
		})
		extra := createEvent(t, ctx, f1.ID, 2024, nil, "Season review")

		r, err := sports.SyncSeason(ctx, syncRepo(), f1.ID, 2024, calendar2024())
		require.NoError(t, err)
		assert.Equal(t, 1, r.EventsCreated, "only Bahrain is new")
		assert.Equal(t, 1, r.EventsUpdated)
		assert.Equal(t, 6, r.SessionsCreated, "Bahrain's three, plus Brazil's sprint qualifying, sprint and qualifying")
		assert.Equal(t, 1, r.SessionsUpdated, "Brazil's existing race got its time and winner")

		adopted, err := db.Event.Find(ctx, mine.ID)
		require.NoError(t, err)
		assert.Equal(t, "2024/interlagos/sao_paulo_grand_prix", adopted.ExternalID)
		assert.Equal(t, "Brazil 2024", adopted.Name, "user's name kept")
		assert.Equal(t, "Autódromo José Carlos Pace", adopted.Circuit, "blank fields filled in")

		race := sessionOfType(t, ctx, mine.ID, models.SessionTypeRace)
		assert.Equal(t, myRace.ID, race.ID, "existing session reused")
		assert.Equal(t, "Max Verstappen", race.Winner)
		assert.NotNil(t, race.StartTime)

		got, err := db.Scene.Find(ctx, rec.ID)
		require.NoError(t, err)
		assert.Equal(t, mine.ID, *got.EventID)
		assert.Equal(t, myRace.ID, *got.SessionID)

		kept, err := db.Event.Find(ctx, extra.ID)
		require.NoError(t, err)
		assert.NotNil(t, kept, "events missing from the source are kept")
	})

	runWithRollbackTxn(t, "renumbered rounds", func(t *testing.T, ctx context.Context) {
		f1, err := sports.EnsureSeries(ctx, syncRepo(), sports.Formula1)
		require.NoError(t, err)

		cal := []sports.CalendarEvent{
			{Season: 2023, Round: 6, Name: "Emilia Romagna Grand Prix", ExternalID: "2023/imola/emilia_romagna_grand_prix", StartDate: "2023-05-19", EndDate: "2023-05-21"},
			{Season: 2023, Round: 7, Name: "Monaco Grand Prix", ExternalID: "2023/monaco/monaco_grand_prix", StartDate: "2023-05-26", EndDate: "2023-05-28"},
			{Season: 2023, Round: 8, Name: "Spanish Grand Prix", ExternalID: "2023/catalunya/spanish_grand_prix", StartDate: "2023-06-02", EndDate: "2023-06-04"},
		}
		_, err = sports.SyncSeason(ctx, syncRepo(), f1.ID, 2023, cal)
		require.NoError(t, err)

		// Imola is cancelled and the later races move up a round
		cal = []sports.CalendarEvent{cal[1], cal[2]}
		cal[0].Round, cal[1].Round = 6, 7
		r, err := sports.SyncSeason(ctx, syncRepo(), f1.ID, 2023, cal)
		require.NoError(t, err)
		assert.Equal(t, 2, r.EventsUpdated)

		season := 2023
		events, err := db.Event.Query(ctx, models.EventQueryOptions{SeriesID: &f1.ID, Season: &season})
		require.NoError(t, err)
		require.Len(t, events, 3)
		assert.Equal(t, "Monaco Grand Prix", events[0].Name)
		assert.Equal(t, 6, *events[0].Round)
		assert.Equal(t, "Spanish Grand Prix", events[1].Name)
		assert.Equal(t, 7, *events[1].Round)
		assert.Equal(t, "Emilia Romagna Grand Prix", events[2].Name, "cancelled event kept, without a round")
		assert.Nil(t, events[2].Round)
	})
}
