//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stashapp/stash/pkg/models"
)

func mustDate(t *testing.T, s string) *models.Date {
	t.Helper()
	d, err := models.ParseDate(s)
	require.NoError(t, err)
	return &d
}

func createSeries(t *testing.T, ctx context.Context, name string) *models.Series {
	t.Helper()
	s := models.NewSeries()
	s.Name = name
	require.NoError(t, db.Series.Create(ctx, &s))
	return &s
}

func createEvent(t *testing.T, ctx context.Context, seriesID, season int, round *int, name string) *models.Event {
	t.Helper()
	e := models.NewEvent()
	e.SeriesID = seriesID
	e.Season = season
	e.Round = round
	e.Name = name
	require.NoError(t, db.Event.Create(ctx, &e))
	return &e
}

func createSession(t *testing.T, ctx context.Context, eventID int, st models.SessionType, start *time.Time) *models.EventSession {
	t.Helper()
	s := models.NewEventSession()
	s.EventID = eventID
	s.SessionType = st
	s.StartTime = start
	require.NoError(t, db.EventSession.Create(ctx, &s))
	return &s
}

func createRecording(t *testing.T, ctx context.Context, title string, mod func(s *models.Scene)) *models.Scene {
	t.Helper()
	s := models.NewScene()
	s.Title = title
	if mod != nil {
		mod(&s)
	}
	require.NoError(t, db.Scene.Create(ctx, &s, nil))
	return &s
}

func TestSeriesStore(t *testing.T) {
	runWithRollbackTxn(t, "crud", func(t *testing.T, ctx context.Context) {
		qb := db.Series

		s := models.NewSeries()
		s.Name = "Formula 1"
		s.ShortName = "F1"
		s.Color = "#E8473C"
		s.DataSource = "jolpica-f1"
		require.NoError(t, qb.Create(ctx, &s))
		assert.NotZero(t, s.ID)
		assert.Equal(t, models.DefaultSport, s.Sport)

		found, err := qb.FindByName(ctx, "formula 1")
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, s.ID, found.ID)
		assert.Equal(t, "F1", found.ShortName)
		assert.Equal(t, "jolpica-f1", found.DataSource)

		missing, err := qb.Find(ctx, s.ID+1000)
		require.NoError(t, err)
		assert.Nil(t, missing)

		p := models.NewSeriesPartial()
		p.ShortName = models.OptionalString{Set: true, Null: true}
		p.Sport = models.NewOptionalString("Motorcycle racing")
		updated, err := qb.UpdatePartial(ctx, s.ID, p)
		require.NoError(t, err)
		assert.Equal(t, "", updated.ShortName)
		assert.Equal(t, "Motorcycle racing", updated.Sport)
		assert.Equal(t, "Formula 1", updated.Name)

		createSeries(t, ctx, "MotoGP")
		all, err := qb.All(ctx)
		require.NoError(t, err)
		require.Len(t, all, 2)
		assert.Equal(t, "Formula 1", all[0].Name)
		assert.Equal(t, "MotoGP", all[1].Name)

		many, err := qb.FindMany(ctx, []int{all[1].ID, all[0].ID})
		require.NoError(t, err)
		assert.Equal(t, "MotoGP", many[0].Name)

		_, err = qb.FindMany(ctx, []int{all[0].ID, 99999})
		assert.Error(t, err)

		dup := models.NewSeries()
		dup.Name = "Formula 1"
		assert.Error(t, qb.Create(ctx, &dup), "series names are unique")
	})
}

func TestEventStore(t *testing.T) {
	runWithRollbackTxn(t, "query", func(t *testing.T, ctx context.Context) {
		f1 := createSeries(t, ctx, "Formula 1")
		motogp := createSeries(t, ctx, "MotoGP")

		r2 := createEvent(t, ctx, f1.ID, 2024, intPtr(2), "Saudi Arabian Grand Prix")
		r1 := createEvent(t, ctx, f1.ID, 2024, intPtr(1), "Bahrain Grand Prix")
		test := createEvent(t, ctx, f1.ID, 2024, nil, "Pre-season testing")
		old := createEvent(t, ctx, f1.ID, 1998, intPtr(16), "Japanese Grand Prix")
		createEvent(t, ctx, motogp.ID, 2015, intPtr(18), "Valencia Grand Prix")

		season := 2024
		events, err := db.Event.Query(ctx, models.EventQueryOptions{SeriesID: &f1.ID, Season: &season})
		require.NoError(t, err)
		require.Len(t, events, 3)
		assert.Equal(t, []int{r1.ID, r2.ID, test.ID}, []int{events[0].ID, events[1].ID, events[2].ID}, "rounds first, unnumbered events last")

		events, err = db.Event.Query(ctx, models.EventQueryOptions{SeriesID: &f1.ID})
		require.NoError(t, err)
		require.Len(t, events, 4)
		assert.Equal(t, old.ID, events[0].ID, "ordered by season")

		seasons, err := db.Event.Seasons(ctx, f1.ID)
		require.NoError(t, err)
		assert.Equal(t, []int{2024, 1998}, seasons)

		n, err := db.Event.CountBySeriesID(ctx, motogp.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		dup := models.NewEvent()
		dup.SeriesID, dup.Season, dup.Round, dup.Name = f1.ID, 2024, intPtr(1), "Duplicate round"
		assert.Error(t, db.Event.Create(ctx, &dup), "one event per series, season and round")
	})

	runWithRollbackTxn(t, "fields and partial update", func(t *testing.T, ctx context.Context) {
		f1 := createSeries(t, ctx, "Formula 1")

		e := models.NewEvent()
		e.SeriesID, e.Season, e.Round, e.Name = f1.ID, 2024, intPtr(21), "São Paulo Grand Prix"
		e.Circuit = "Autódromo José Carlos Pace"
		e.Location = "São Paulo"
		e.Country = "Brazil"
		e.StartDate = mustDate(t, "2024-11-01")
		e.EndDate = mustDate(t, "2024-11-03")
		e.ExternalID = "2024/21"
		require.NoError(t, db.Event.Create(ctx, &e))

		found, err := db.Event.FindByExternalID(ctx, f1.ID, "2024/21")
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, "Autódromo José Carlos Pace", found.Circuit)
		assert.Equal(t, "2024-11-01", found.StartDate.String())
		assert.Equal(t, "2024-11-03", found.EndDate.String())
		assert.Equal(t, 21, *found.Round)

		none, err := db.Event.FindByExternalID(ctx, f1.ID+1, "2024/21")
		require.NoError(t, err)
		assert.Nil(t, none)

		p := models.NewEventPartial()
		p.Round = models.OptionalInt{Set: true, Null: true}
		p.EndDate = models.OptionalDate{Set: true, Null: true}
		p.Name = models.NewOptionalString("Brazilian Grand Prix")
		updated, err := db.Event.UpdatePartial(ctx, e.ID, p)
		require.NoError(t, err)
		assert.Nil(t, updated.Round)
		assert.Nil(t, updated.EndDate)
		assert.Equal(t, "2024-11-01", updated.StartDate.String(), "unchanged fields are kept")
		assert.Equal(t, "Brazilian Grand Prix", updated.Name)
	})
}

func TestEventSessionStore(t *testing.T) {
	runWithRollbackTxn(t, "ordering", func(t *testing.T, ctx context.Context) {
		f1 := createSeries(t, ctx, "Formula 1")
		e := createEvent(t, ctx, f1.ID, 2024, intPtr(21), "São Paulo Grand Prix")

		race := createSession(t, ctx, e.ID, models.SessionTypeRace, nil)
		fp1 := createSession(t, ctx, e.ID, models.SessionTypePractice1, nil)
		quali := createSession(t, ctx, e.ID, models.SessionTypeQualifying, nil)
		sprint := createSession(t, ctx, e.ID, models.SessionTypeSprint, nil)

		sessions, err := db.EventSession.FindByEventID(ctx, e.ID)
		require.NoError(t, err)
		var got []int
		for _, s := range sessions {
			got = append(got, s.ID)
		}
		assert.Equal(t, []int{fp1.ID, quali.ID, sprint.ID, race.ID}, got, "without times, sessions follow weekend order")

		// 2024 São Paulo qualifying was moved to Sunday morning, after the sprint
		sat := time.Date(2024, 11, 2, 14, 30, 0, 0, time.UTC)
		sun := time.Date(2024, 11, 3, 10, 30, 0, 0, time.UTC)
		p := models.NewEventSessionPartial()
		p.StartTime = models.NewOptionalTime(sun)
		_, err = db.EventSession.UpdatePartial(ctx, quali.ID, p)
		require.NoError(t, err)
		p.StartTime = models.NewOptionalTime(sat)
		_, err = db.EventSession.UpdatePartial(ctx, sprint.ID, p)
		require.NoError(t, err)

		sessions, err = db.EventSession.FindByEventID(ctx, e.ID)
		require.NoError(t, err)
		got = got[:0]
		for _, s := range sessions {
			got = append(got, s.ID)
		}
		assert.Equal(t, []int{sprint.ID, quali.ID, fp1.ID, race.ID}, got, "scheduled sessions come first, in time order")

		found, err := db.EventSession.Find(ctx, quali.ID)
		require.NoError(t, err)
		require.NotNil(t, found.StartTime)
		assert.True(t, found.StartTime.Equal(sun))
		assert.Equal(t, models.SessionTypeQualifying, found.SessionType)
	})
}

func TestSceneSportsFields(t *testing.T) {
	runWithRollbackTxn(t, "round trip", func(t *testing.T, ctx context.Context) {
		f1 := createSeries(t, ctx, "Formula 1")
		e := createEvent(t, ctx, f1.ID, 2024, intPtr(21), "São Paulo Grand Prix")
		race := createSession(t, ctx, e.ID, models.SessionTypeRace, nil)

		feed := models.FeedTypeOnboard
		source := models.RecordingSourceStreaming
		complete := models.CompletenessComplete
		offset := 0.0
		s := createRecording(t, ctx, "Onboard VER", func(s *models.Scene) {
			s.EventID = &e.ID
			s.SessionID = &race.ID
			s.Language = "en"
			s.FeedType = &feed
			s.FeedSubject = "Max Verstappen"
			s.Source = &source
			s.Completeness = &complete
			s.Commentators = "David Croft, Martin Brundle"
			s.SyncOffset = &offset
		})

		found, err := db.Scene.Find(ctx, s.ID)
		require.NoError(t, err)
		assert.Equal(t, e.ID, *found.EventID)
		assert.Equal(t, race.ID, *found.SessionID)
		assert.Equal(t, "en", found.Language)
		assert.Equal(t, models.FeedTypeOnboard, *found.FeedType)
		assert.Equal(t, "Max Verstappen", found.FeedSubject)
		assert.Equal(t, models.RecordingSourceStreaming, *found.Source)
		assert.Equal(t, models.CompletenessComplete, *found.Completeness)
		assert.Equal(t, "David Croft, Martin Brundle", found.Commentators)
		require.NotNil(t, found.SyncOffset, "a zero offset is a valid sync point")
		assert.Equal(t, 0.0, *found.SyncOffset)

		p := models.NewScenePartial()
		p.FeedType = models.OptionalString{Set: true, Null: true}
		p.SyncOffset = models.NewOptionalFloat64(2190.5)
		p.Language = models.NewOptionalString("de")
		updated, err := db.Scene.UpdatePartial(ctx, s.ID, p)
		require.NoError(t, err)
		assert.Nil(t, updated.FeedType)
		assert.Equal(t, 2190.5, *updated.SyncOffset)
		assert.Equal(t, "de", updated.Language)
		assert.Equal(t, models.RecordingSourceStreaming, *updated.Source, "unchanged fields are kept")

		plain := createRecording(t, ctx, "Unlinked recording", nil)
		found, err = db.Scene.Find(ctx, plain.ID)
		require.NoError(t, err)
		assert.Nil(t, found.EventID)
		assert.Nil(t, found.FeedType)
		assert.Nil(t, found.SyncOffset)
	})

	runWithRollbackTxn(t, "deleting calendar entries keeps recordings", func(t *testing.T, ctx context.Context) {
		f1 := createSeries(t, ctx, "Formula 1")
		e := createEvent(t, ctx, f1.ID, 2024, intPtr(21), "São Paulo Grand Prix")
		race := createSession(t, ctx, e.ID, models.SessionTypeRace, nil)
		s := createRecording(t, ctx, "Race", func(s *models.Scene) {
			s.EventID = &e.ID
			s.SessionID = &race.ID
		})

		require.NoError(t, db.EventSession.Destroy(ctx, race.ID))
		found, err := db.Scene.Find(ctx, s.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Nil(t, found.SessionID)
		assert.Equal(t, e.ID, *found.EventID, "recording stays with its event")

		require.NoError(t, db.Series.Destroy(ctx, f1.ID))
		gone, err := db.Event.Find(ctx, e.ID)
		require.NoError(t, err)
		assert.Nil(t, gone, "events are deleted with their series")

		found, err = db.Scene.Find(ctx, s.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Nil(t, found.EventID)
	})
}

func TestSceneSportsFilters(t *testing.T) {
	runWithRollbackTxn(t, "filters", func(t *testing.T, ctx context.Context) {
		f1 := createSeries(t, ctx, "Formula 1")
		motogp := createSeries(t, ctx, "MotoGP")
		brazil := createEvent(t, ctx, f1.ID, 2024, intPtr(21), "São Paulo Grand Prix")
		suzuka := createEvent(t, ctx, f1.ID, 1998, intPtr(16), "Japanese Grand Prix")
		valencia := createEvent(t, ctx, motogp.ID, 2015, intPtr(18), "Valencia Grand Prix")
		race := createSession(t, ctx, brazil.ID, models.SessionTypeRace, nil)
		quali := createSession(t, ctx, brazil.ID, models.SessionTypeQualifying, nil)
		suzukaRace := createSession(t, ctx, suzuka.ID, models.SessionTypeRace, nil)

		world, onboard := models.FeedTypeWorldFeed, models.FeedTypeOnboard
		tv, dvd := models.RecordingSourceTV, models.RecordingSourceDVD
		partial := models.CompletenessPartial

		link := func(e *models.Event, s *models.EventSession) func(*models.Scene) {
			return func(sc *models.Scene) {
				sc.EventID = &e.ID
				if s != nil {
					sc.SessionID = &s.ID
				}
			}
		}
		with := func(fns ...func(*models.Scene)) func(*models.Scene) {
			return func(sc *models.Scene) {
				for _, fn := range fns {
					fn(sc)
				}
			}
		}

		skyRace := createRecording(t, ctx, "sports-test Sky race", with(link(brazil, race), func(s *models.Scene) {
			s.Language, s.FeedType, s.Source = "en", &world, &tv
		}))
		onboardRace := createRecording(t, ctx, "sports-test onboard", with(link(brazil, race), func(s *models.Scene) {
			s.FeedType, s.FeedSubject = &onboard, "Max Verstappen"
		}))
		skyQuali := createRecording(t, ctx, "sports-test Sky quali", with(link(brazil, quali), func(s *models.Scene) {
			s.Language, s.FeedType, s.Completeness = "en", &world, &partial
		}))
		rtl := createRecording(t, ctx, "sports-test RTL 1998", with(link(suzuka, suzukaRace), func(s *models.Scene) {
			s.Language, s.FeedType, s.Source = "de", &world, &dvd
		}))
		motoRace := createRecording(t, ctx, "sports-test MotoGP", with(link(valencia, nil), func(s *models.Scene) {
			s.Language = "en"
		}))
		unlinked := createRecording(t, ctx, "sports-test unlinked", nil)

		all := []int{skyRace.ID, onboardRace.ID, skyQuali.ID, rtl.ID, motoRace.ID, unlinked.ID}

		query := func(t *testing.T, f *models.SceneFilterType) []int {
			t.Helper()
			title := "sports-test"
			f.Title = &models.StringCriterionInput{Value: title, Modifier: models.CriterionModifierIncludes}
			var ret []int
			for _, s := range queryScene(ctx, t, db.Scene, f, nil) {
				ret = append(ret, s.ID)
			}
			return ret
		}
		ids := func(v ...int) []string {
			var ret []string
			for _, i := range v {
				ret = append(ret, strconv.Itoa(i))
			}
			return ret
		}
		incl, excl := models.CriterionModifierIncludes, models.CriterionModifierExcludes

		tests := []struct {
			name   string
			filter models.SceneFilterType
			want   []int
		}{
			{"series", models.SceneFilterType{Series: &models.MultiCriterionInput{Value: ids(f1.ID), Modifier: incl}},
				[]int{skyRace.ID, onboardRace.ID, skyQuali.ID, rtl.ID}},
			{"series excludes keeps unlinked", models.SceneFilterType{Series: &models.MultiCriterionInput{Value: ids(f1.ID), Modifier: excl}},
				[]int{motoRace.ID, unlinked.ID}},
			{"events", models.SceneFilterType{Events: &models.MultiCriterionInput{Value: ids(suzuka.ID, valencia.ID), Modifier: incl}},
				[]int{rtl.ID, motoRace.ID}},
			{"no event", models.SceneFilterType{Events: &models.MultiCriterionInput{Modifier: models.CriterionModifierIsNull}},
				[]int{unlinked.ID}},
			{"sessions", models.SceneFilterType{Sessions: &models.MultiCriterionInput{Value: ids(race.ID), Modifier: incl}},
				[]int{skyRace.ID, onboardRace.ID}},
			{"season", models.SceneFilterType{Season: &models.IntCriterionInput{Value: 2000, Modifier: models.CriterionModifierGreaterThan}},
				[]int{skyRace.ID, onboardRace.ID, skyQuali.ID, motoRace.ID}},
			{"session type", models.SceneFilterType{SessionType: &models.SessionTypeCriterionInput{Value: []models.SessionType{models.SessionTypeRace}, Modifier: incl}},
				[]int{skyRace.ID, onboardRace.ID, rtl.ID}},
			{"session type excludes keeps sessionless", models.SceneFilterType{SessionType: &models.SessionTypeCriterionInput{Value: []models.SessionType{models.SessionTypeRace}, Modifier: excl}},
				[]int{skyQuali.ID, motoRace.ID, unlinked.ID}},
			{"language", models.SceneFilterType{Language: &models.StringCriterionInput{Value: "en", Modifier: models.CriterionModifierEquals}},
				[]int{skyRace.ID, skyQuali.ID, motoRace.ID}},
			{"feed type", models.SceneFilterType{FeedType: &models.FeedTypeCriterionInput{Value: []models.FeedType{models.FeedTypeOnboard}, Modifier: incl}},
				[]int{onboardRace.ID}},
			{"feed type excludes", models.SceneFilterType{FeedType: &models.FeedTypeCriterionInput{Value: []models.FeedType{models.FeedTypeWorldFeed}, Modifier: excl}},
				[]int{onboardRace.ID, motoRace.ID, unlinked.ID}},
			{"feed subject", models.SceneFilterType{FeedSubject: &models.StringCriterionInput{Value: "verstappen", Modifier: models.CriterionModifierIncludes}},
				[]int{onboardRace.ID}},
			{"source", models.SceneFilterType{Source: &models.RecordingSourceCriterionInput{Value: []models.RecordingSource{models.RecordingSourceDVD}, Modifier: incl}},
				[]int{rtl.ID}},
			{"completeness", models.SceneFilterType{Completeness: &models.CompletenessCriterionInput{Value: []models.Completeness{models.CompletenessPartial}, Modifier: incl}},
				[]int{skyQuali.ID}},
			{"no source", models.SceneFilterType{Source: &models.RecordingSourceCriterionInput{Modifier: models.CriterionModifierIsNull}},
				[]int{onboardRace.ID, skyQuali.ID, motoRace.ID, unlinked.ID}},
			{"combined", models.SceneFilterType{
				Series:      &models.MultiCriterionInput{Value: ids(f1.ID), Modifier: incl},
				SessionType: &models.SessionTypeCriterionInput{Value: []models.SessionType{models.SessionTypeRace}, Modifier: incl},
				Language:    &models.StringCriterionInput{Value: "en", Modifier: models.CriterionModifierEquals},
			}, []int{skyRace.ID}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				f := tt.filter
				assert.ElementsMatch(t, tt.want, query(t, &f))
			})
		}

		assert.ElementsMatch(t, all, query(t, &models.SceneFilterType{}))

		p := models.NewScenePartial()
		p.ImportDismissed = models.NewOptionalBool(true)
		_, err := db.Scene.UpdatePartial(ctx, unlinked.ID, p)
		require.NoError(t, err)
		yes, no := true, false
		assert.ElementsMatch(t, []int{unlinked.ID}, query(t, &models.SceneFilterType{ImportDismissed: &yes}))
		assert.NotContains(t, query(t, &models.SceneFilterType{ImportDismissed: &no}), unlinked.ID)
	})
}
