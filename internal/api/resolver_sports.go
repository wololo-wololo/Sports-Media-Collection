package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/models"
)

type seriesResolver struct{ *Resolver }
type eventResolver struct{ *Resolver }
type eventSessionResolver struct{ *Resolver }

func (r *Resolver) Series() SeriesResolver {
	return &seriesResolver{r}
}

func (r *Resolver) Event() EventResolver {
	return &eventResolver{r}
}

func (r *Resolver) EventSession() EventSessionResolver {
	return &eventSessionResolver{r}
}

func parseID(id string, kind string) (int, error) {
	ret, err := strconv.Atoi(id)
	if err != nil {
		return 0, fmt.Errorf("converting %s id: %w", kind, err)
	}
	return ret, nil
}

func trimmedPtr(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	return &s
}

// ---------------------------------------------------------------------------
// queries

func (r *queryResolver) AllSeries(ctx context.Context) (ret []*models.Series, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Series.All(ctx)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *queryResolver) FindSeries(ctx context.Context, id string) (ret *models.Series, err error) {
	idInt, err := parseID(id, "series")
	if err != nil {
		return nil, err
	}

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Series.Find(ctx, idInt)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *queryResolver) FindEvents(ctx context.Context, seriesID *string, season *int) (ret []*models.Event, err error) {
	var options models.EventQueryOptions
	if seriesID != nil {
		id, err := parseID(*seriesID, "series")
		if err != nil {
			return nil, err
		}
		options.SeriesID = &id
	}
	options.Season = season

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Event.Query(ctx, options)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *queryResolver) FindEvent(ctx context.Context, id string) (ret *models.Event, err error) {
	idInt, err := parseID(id, "event")
	if err != nil {
		return nil, err
	}

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Event.Find(ctx, idInt)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *queryResolver) FindEventSession(ctx context.Context, id string) (ret *models.EventSession, err error) {
	idInt, err := parseID(id, "event session")
	if err != nil {
		return nil, err
	}

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.EventSession.Find(ctx, idInt)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

// ---------------------------------------------------------------------------
// series mutations

func (r *mutationResolver) SeriesCreate(ctx context.Context, input SeriesCreateInput) (*models.Series, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("series name must not be empty")
	}

	translator := changesetTranslator{inputMap: getUpdateInputMap(ctx)}

	newSeries := models.NewSeries()
	newSeries.Name = name
	newSeries.ShortName = translator.string(trimmedPtr(input.ShortName))
	if sport := translator.string(trimmedPtr(input.Sport)); sport != "" {
		newSeries.Sport = sport
	}
	newSeries.Color = translator.string(trimmedPtr(input.Color))
	newSeries.DataSource = translator.string(trimmedPtr(input.DataSource))

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		existing, err := r.repository.Series.FindByName(ctx, name)
		if err != nil {
			return err
		}
		if existing != nil {
			return fmt.Errorf("series %q already exists", name)
		}
		return r.repository.Series.Create(ctx, &newSeries)
	}); err != nil {
		return nil, err
	}

	return &newSeries, nil
}

func (r *mutationResolver) SeriesUpdate(ctx context.Context, input SeriesUpdateInput) (ret *models.Series, err error) {
	id, err := parseID(input.ID, "series")
	if err != nil {
		return nil, err
	}

	translator := changesetTranslator{inputMap: getUpdateInputMap(ctx)}

	partial := models.NewSeriesPartial()
	partial.Name = translator.optionalString(trimmedPtr(input.Name), "name")
	partial.ShortName = translator.optionalString(trimmedPtr(input.ShortName), "short_name")
	partial.Sport = translator.optionalString(trimmedPtr(input.Sport), "sport")
	partial.Color = translator.optionalString(trimmedPtr(input.Color), "color")
	partial.DataSource = translator.optionalString(trimmedPtr(input.DataSource), "data_source")

	if partial.Name.Set && (partial.Name.Null || partial.Name.Value == "") {
		return nil, errors.New("series name must not be empty")
	}
	if partial.Sport.Set && (partial.Sport.Null || partial.Sport.Value == "") {
		partial.Sport = models.NewOptionalString(models.DefaultSport)
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.Series
		existing, err := qb.Find(ctx, id)
		if err != nil {
			return err
		}
		if existing == nil {
			return fmt.Errorf("series with id %d not found", id)
		}

		if partial.Name.Set {
			other, err := qb.FindByName(ctx, partial.Name.Value)
			if err != nil {
				return err
			}
			if other != nil && other.ID != id {
				return fmt.Errorf("series %q already exists", partial.Name.Value)
			}
		}

		ret, err = qb.UpdatePartial(ctx, id, partial)
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) SeriesDestroy(ctx context.Context, id string) (bool, error) {
	idInt, err := parseID(id, "series")
	if err != nil {
		return false, err
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		return r.repository.Series.Destroy(ctx, idInt)
	}); err != nil {
		return false, err
	}

	return true, nil
}

// ---------------------------------------------------------------------------
// event mutations

func validateEventDates(start, end *models.Date) error {
	if start != nil && end != nil && end.Time.Before(start.Time) {
		return errors.New("event end date must not be before its start date")
	}
	return nil
}

func (r *mutationResolver) EventCreate(ctx context.Context, input EventCreateInput) (*models.Event, error) {
	seriesID, err := parseID(input.SeriesID, "series")
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("event name must not be empty")
	}

	translator := changesetTranslator{inputMap: getUpdateInputMap(ctx)}

	newEvent := models.NewEvent()
	newEvent.SeriesID = seriesID
	newEvent.Season = input.Season
	newEvent.Round = input.Round
	newEvent.Name = name
	newEvent.Circuit = translator.string(trimmedPtr(input.Circuit))
	newEvent.Location = translator.string(trimmedPtr(input.Location))
	newEvent.Country = translator.string(trimmedPtr(input.Country))
	newEvent.ExternalID = translator.string(trimmedPtr(input.ExternalID))

	newEvent.StartDate, err = translator.datePtr(input.StartDate)
	if err != nil {
		return nil, fmt.Errorf("converting start date: %w", err)
	}
	newEvent.EndDate, err = translator.datePtr(input.EndDate)
	if err != nil {
		return nil, fmt.Errorf("converting end date: %w", err)
	}
	if err := validateEventDates(newEvent.StartDate, newEvent.EndDate); err != nil {
		return nil, err
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		series, err := r.repository.Series.Find(ctx, seriesID)
		if err != nil {
			return err
		}
		if series == nil {
			return fmt.Errorf("series with id %d not found", seriesID)
		}
		return r.repository.Event.Create(ctx, &newEvent)
	}); err != nil {
		return nil, err
	}

	return &newEvent, nil
}

func (r *mutationResolver) EventUpdate(ctx context.Context, input EventUpdateInput) (ret *models.Event, err error) {
	id, err := parseID(input.ID, "event")
	if err != nil {
		return nil, err
	}

	translator := changesetTranslator{inputMap: getUpdateInputMap(ctx)}

	partial := models.NewEventPartial()
	partial.SeriesID, err = translator.optionalIntFromString(input.SeriesID, "series_id")
	if err != nil {
		return nil, fmt.Errorf("converting series id: %w", err)
	}
	if partial.SeriesID.Set && partial.SeriesID.Null {
		return nil, errors.New("an event must belong to a series")
	}
	partial.Season = translator.optionalInt(input.Season, "season")
	if partial.Season.Set && partial.Season.Null {
		return nil, errors.New("an event must have a season")
	}
	partial.Round = translator.optionalInt(input.Round, "round")
	partial.Name = translator.optionalString(trimmedPtr(input.Name), "name")
	if partial.Name.Set && (partial.Name.Null || partial.Name.Value == "") {
		return nil, errors.New("event name must not be empty")
	}
	partial.Circuit = translator.optionalString(trimmedPtr(input.Circuit), "circuit")
	partial.Location = translator.optionalString(trimmedPtr(input.Location), "location")
	partial.Country = translator.optionalString(trimmedPtr(input.Country), "country")
	partial.ExternalID = translator.optionalString(trimmedPtr(input.ExternalID), "external_id")
	partial.StartDate, err = translator.optionalDate(input.StartDate, "start_date")
	if err != nil {
		return nil, fmt.Errorf("converting start date: %w", err)
	}
	partial.EndDate, err = translator.optionalDate(input.EndDate, "end_date")
	if err != nil {
		return nil, fmt.Errorf("converting end date: %w", err)
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.Event
		existing, err := qb.Find(ctx, id)
		if err != nil {
			return err
		}
		if existing == nil {
			return fmt.Errorf("event with id %d not found", id)
		}

		if partial.SeriesID.Set {
			series, err := r.repository.Series.Find(ctx, partial.SeriesID.Value)
			if err != nil {
				return err
			}
			if series == nil {
				return fmt.Errorf("series with id %d not found", partial.SeriesID.Value)
			}
		}

		start, end := existing.StartDate, existing.EndDate
		if partial.StartDate.Set {
			start = partial.StartDate.Ptr()
		}
		if partial.EndDate.Set {
			end = partial.EndDate.Ptr()
		}
		if err := validateEventDates(start, end); err != nil {
			return err
		}

		ret, err = qb.UpdatePartial(ctx, id, partial)
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) EventDestroy(ctx context.Context, id string) (bool, error) {
	idInt, err := parseID(id, "event")
	if err != nil {
		return false, err
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		return r.repository.Event.Destroy(ctx, idInt)
	}); err != nil {
		return false, err
	}

	return true, nil
}

// ---------------------------------------------------------------------------
// event session mutations

func (r *mutationResolver) EventSessionCreate(ctx context.Context, input EventSessionCreateInput) (*models.EventSession, error) {
	eventID, err := parseID(input.EventID, "event")
	if err != nil {
		return nil, err
	}

	translator := changesetTranslator{inputMap: getUpdateInputMap(ctx)}

	newSession := models.NewEventSession()
	newSession.EventID = eventID
	newSession.SessionType = input.SessionType
	newSession.Name = translator.string(trimmedPtr(input.Name))
	newSession.StartTime = input.StartTime
	newSession.Winner = translator.string(trimmedPtr(input.Winner))

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		event, err := r.repository.Event.Find(ctx, eventID)
		if err != nil {
			return err
		}
		if event == nil {
			return fmt.Errorf("event with id %d not found", eventID)
		}
		return r.repository.EventSession.Create(ctx, &newSession)
	}); err != nil {
		return nil, err
	}

	return &newSession, nil
}

func (r *mutationResolver) EventSessionUpdate(ctx context.Context, input EventSessionUpdateInput) (ret *models.EventSession, err error) {
	id, err := parseID(input.ID, "event session")
	if err != nil {
		return nil, err
	}

	translator := changesetTranslator{inputMap: getUpdateInputMap(ctx)}

	partial := models.NewEventSessionPartial()
	partial.EventID, err = translator.optionalIntFromString(input.EventID, "event_id")
	if err != nil {
		return nil, fmt.Errorf("converting event id: %w", err)
	}
	if partial.EventID.Set && partial.EventID.Null {
		return nil, errors.New("a session must belong to an event")
	}
	partial.SessionType = translator.optionalString(enumStringPtr(input.SessionType), "session_type")
	if partial.SessionType.Set && partial.SessionType.Null {
		return nil, errors.New("a session must have a session type")
	}
	partial.Name = translator.optionalString(trimmedPtr(input.Name), "name")
	partial.Winner = translator.optionalString(trimmedPtr(input.Winner), "winner")
	if translator.hasField("start_time") {
		partial.StartTime = models.NewOptionalTimePtr(input.StartTime)
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		qb := r.repository.EventSession
		existing, err := qb.Find(ctx, id)
		if err != nil {
			return err
		}
		if existing == nil {
			return fmt.Errorf("event session with id %d not found", id)
		}

		if partial.EventID.Set && partial.EventID.Value != existing.EventID {
			event, err := r.repository.Event.Find(ctx, partial.EventID.Value)
			if err != nil {
				return err
			}
			if event == nil {
				return fmt.Errorf("event with id %d not found", partial.EventID.Value)
			}
			// moving a session moves its recordings with it
			if err := r.moveSessionRecordings(ctx, id, partial.EventID.Value); err != nil {
				return err
			}
		}

		ret, err = qb.UpdatePartial(ctx, id, partial)
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

// moveSessionRecordings sets the event of all recordings of a session.
func (r *mutationResolver) moveSessionRecordings(ctx context.Context, sessionID int, eventID int) error {
	scenes, err := r.scenesByFilter(ctx, sessionScenesFilter(sessionID))
	if err != nil {
		return err
	}

	for _, s := range scenes {
		partial := models.NewScenePartial()
		partial.EventID = models.NewOptionalInt(eventID)
		if _, err := r.repository.Scene.UpdatePartial(ctx, s.ID, partial); err != nil {
			return err
		}
	}
	return nil
}

func (r *mutationResolver) EventSessionDestroy(ctx context.Context, id string) (bool, error) {
	idInt, err := parseID(id, "event session")
	if err != nil {
		return false, err
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		return r.repository.EventSession.Destroy(ctx, idInt)
	}); err != nil {
		return false, err
	}

	return true, nil
}

// ---------------------------------------------------------------------------
// calendar sync

// maxSyncSeasons bounds a single request; a full history is about 80.
const maxSyncSeasons = 200

func (r *mutationResolver) SyncSeriesCalendar(ctx context.Context, input SyncSeriesCalendarInput) (string, error) {
	if len(input.Seasons) > maxSyncSeasons {
		return "", fmt.Errorf("at most %d seasons can be synced at once", maxSyncSeasons)
	}

	seen := make(map[int]bool)
	var seasons []int
	for _, y := range input.Seasons {
		if y < 1900 || y > 2200 {
			return "", fmt.Errorf("invalid season %d", y)
		}
		if !seen[y] {
			seen[y] = true
			seasons = append(seasons, y)
		}
	}

	jobID := manager.GetInstance().SyncCalendar(ctx, seasons)
	return strconv.Itoa(jobID), nil
}

// ---------------------------------------------------------------------------
// model resolvers

// scenesByFilter returns all scenes matching filter, ordered by title.
func (r *Resolver) scenesByFilter(ctx context.Context, filter *models.SceneFilterType) ([]*models.Scene, error) {
	perPage := models.PerPageAll
	sort := "title"

	result, err := r.repository.Scene.Query(ctx, models.SceneQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: &models.FindFilterType{PerPage: &perPage, Sort: &sort},
		},
		SceneFilter: filter,
	})
	if err != nil {
		return nil, err
	}

	return result.Resolve(ctx)
}

func idFilter(id int) *models.MultiCriterionInput {
	return &models.MultiCriterionInput{
		Value:    []string{strconv.Itoa(id)},
		Modifier: models.CriterionModifierIncludes,
	}
}

func (r *seriesResolver) Seasons(ctx context.Context, obj *models.Series) (ret []int, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Event.Seasons(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	if ret == nil {
		ret = []int{}
	}
	return ret, nil
}

func (r *seriesResolver) SceneCount(ctx context.Context, obj *models.Series) (ret int, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Scene.QueryCount(ctx, &models.SceneFilterType{Series: idFilter(obj.ID)}, nil)
		return err
	}); err != nil {
		return 0, err
	}
	return ret, nil
}

func (r *queryResolver) SeasonCoverage(ctx context.Context, seriesID string, season int) ([]*EventCoverage, error) {
	sid, err := parseID(seriesID, "series")
	if err != nil {
		return nil, err
	}

	ret := []*EventCoverage{}
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		events, err := r.repository.Event.Query(ctx, models.EventQueryOptions{SeriesID: &sid, Season: &season})
		if err != nil {
			return err
		}
		counts, err := r.repository.Event.RecordingCounts(ctx, sid, season)
		if err != nil {
			return err
		}

		bySession := make(map[int]models.RecordingCount)
		extras := make(map[int]int)
		totals := make(map[int]int)
		for _, c := range counts {
			totals[c.EventID] += c.Recordings
			if c.SessionID == nil {
				extras[c.EventID] += c.Recordings
			} else {
				bySession[*c.SessionID] = c
			}
		}

		for _, e := range events {
			sessions, err := r.repository.EventSession.FindByEventID(ctx, e.ID)
			if err != nil {
				return err
			}
			ec := &EventCoverage{Event: e, Sessions: []*SessionCoverage{}, Extras: extras[e.ID], Recordings: totals[e.ID]}
			for _, s := range sessions {
				c := bySession[s.ID]
				ec.Sessions = append(ec.Sessions, &SessionCoverage{Session: s, Recordings: c.Recordings, Partial: c.Partial})
			}
			ret = append(ret, ec)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *queryResolver) RecordingLanguages(ctx context.Context) (ret []string, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Event.RecordingLanguages(ctx)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *queryResolver) SeriesCoverage(ctx context.Context, seriesID string) ([]*SeasonCoverage, error) {
	sid, err := parseID(seriesID, "series")
	if err != nil {
		return nil, err
	}

	ret := []*SeasonCoverage{}
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		stats, err := r.repository.Event.SeasonStats(ctx, sid)
		if err != nil {
			return err
		}
		for _, s := range stats {
			ret = append(ret, &SeasonCoverage{
				Season:          s.Season,
				Events:          s.Events,
				Sessions:        s.Sessions,
				CoveredSessions: s.CoveredSessions,
				Recordings:      s.Recordings,
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *seriesResolver) EventCount(ctx context.Context, obj *models.Series) (ret int, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Event.CountBySeriesID(ctx, obj.ID)
		return err
	}); err != nil {
		return 0, err
	}
	return ret, nil
}

func (r *eventResolver) Series(ctx context.Context, obj *models.Event) (ret *models.Series, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Series.Find(ctx, obj.SeriesID)
		return err
	}); err != nil {
		return nil, err
	}
	if ret == nil {
		return nil, fmt.Errorf("series with id %d not found", obj.SeriesID)
	}
	return ret, nil
}

func (r *eventResolver) StartDate(ctx context.Context, obj *models.Event) (*string, error) {
	if obj.StartDate == nil {
		return nil, nil
	}
	s := obj.StartDate.String()
	return &s, nil
}

func (r *eventResolver) EndDate(ctx context.Context, obj *models.Event) (*string, error) {
	if obj.EndDate == nil {
		return nil, nil
	}
	s := obj.EndDate.String()
	return &s, nil
}

func (r *eventResolver) Sessions(ctx context.Context, obj *models.Event) (ret []*models.EventSession, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.EventSession.FindByEventID(ctx, obj.ID)
		return err
	}); err != nil {
		return nil, err
	}
	if ret == nil {
		ret = []*models.EventSession{}
	}
	return ret, nil
}

func eventScenesFilter(eventID int) *models.SceneFilterType {
	return &models.SceneFilterType{Events: idFilter(eventID)}
}

func sessionScenesFilter(sessionID int) *models.SceneFilterType {
	return &models.SceneFilterType{Sessions: idFilter(sessionID)}
}

func (r *eventResolver) Scenes(ctx context.Context, obj *models.Event) (ret []*models.Scene, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.scenesByFilter(ctx, eventScenesFilter(obj.ID))
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *eventResolver) SceneCount(ctx context.Context, obj *models.Event) (ret int, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Scene.QueryCount(ctx, eventScenesFilter(obj.ID), nil)
		return err
	}); err != nil {
		return 0, err
	}
	return ret, nil
}

func (r *eventSessionResolver) Event(ctx context.Context, obj *models.EventSession) (ret *models.Event, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Event.Find(ctx, obj.EventID)
		return err
	}); err != nil {
		return nil, err
	}
	if ret == nil {
		return nil, fmt.Errorf("event with id %d not found", obj.EventID)
	}
	return ret, nil
}

func (r *eventSessionResolver) Scenes(ctx context.Context, obj *models.EventSession) (ret []*models.Scene, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.scenesByFilter(ctx, sessionScenesFilter(obj.ID))
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *eventSessionResolver) SceneCount(ctx context.Context, obj *models.EventSession) (ret int, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Scene.QueryCount(ctx, sessionScenesFilter(obj.ID), nil)
		return err
	}); err != nil {
		return 0, err
	}
	return ret, nil
}

func (r *sceneResolver) Event(ctx context.Context, obj *models.Scene) (ret *models.Event, err error) {
	if obj.EventID == nil {
		return nil, nil
	}

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Event.Find(ctx, *obj.EventID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *sceneResolver) Session(ctx context.Context, obj *models.Scene) (ret *models.EventSession, err error) {
	if obj.SessionID == nil {
		return nil, nil
	}

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.EventSession.Find(ctx, *obj.SessionID)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}
