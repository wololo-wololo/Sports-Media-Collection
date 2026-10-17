package sqlite

import (
	"context"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
	"gopkg.in/guregu/null.v4"
	"gopkg.in/guregu/null.v4/zero"

	"github.com/stashapp/stash/pkg/models"
)

const (
	seriesTable        = "series"
	eventsTable        = "events"
	eventSessionsTable = "event_sessions"
)

// ---------------------------------------------------------------------------
// shared helpers

// findManyOrdered returns the rows for ids in the same order as ids, failing
// if any id is missing.
func findManyOrdered[T any](ids []int, unsorted []*T, idOf func(*T) int, kind string) ([]*T, error) {
	ret := make([]*T, len(ids))
	for _, s := range unsorted {
		for i, id := range ids {
			if idOf(s) == id {
				ret[i] = s
			}
		}
	}

	for i := range ret {
		if ret[i] == nil {
			return nil, fmt.Errorf("%s with id %d not found", kind, ids[i])
		}
	}

	return ret, nil
}

func scanRows[R any, T any](ctx context.Context, q *goqu.SelectDataset, resolve func(*R) *T) ([]*T, error) {
	const single = false
	var ret []*T
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var row R
		if err := r.StructScan(&row); err != nil {
			return err
		}
		ret = append(ret, resolve(&row))
		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func nullDateValue(d models.OptionalDate) NullDate {
	if d.Null || !d.Set {
		return NullDate{}
	}
	return NullDate{Date: d.Value.Time, Valid: true}
}

// ---------------------------------------------------------------------------
// series

type seriesRow struct {
	ID         int         `db:"id" goqu:"skipinsert"`
	Name       string      `db:"name"`
	ShortName  zero.String `db:"short_name"`
	Sport      string      `db:"sport"`
	Color      zero.String `db:"color"`
	DataSource zero.String `db:"data_source"`
	CreatedAt  Timestamp   `db:"created_at"`
	UpdatedAt  Timestamp   `db:"updated_at"`
}

func (r *seriesRow) fromSeries(o models.Series) {
	r.ID = o.ID
	r.Name = o.Name
	r.ShortName = zero.StringFrom(o.ShortName)
	r.Sport = o.Sport
	if r.Sport == "" {
		r.Sport = models.DefaultSport
	}
	r.Color = zero.StringFrom(o.Color)
	r.DataSource = zero.StringFrom(o.DataSource)
	r.CreatedAt = Timestamp{Timestamp: o.CreatedAt}
	r.UpdatedAt = Timestamp{Timestamp: o.UpdatedAt}
}

func (r *seriesRow) resolve() *models.Series {
	return &models.Series{
		ID:         r.ID,
		Name:       r.Name,
		ShortName:  r.ShortName.String,
		Sport:      r.Sport,
		Color:      r.Color.String,
		DataSource: r.DataSource.String,
		CreatedAt:  r.CreatedAt.Timestamp,
		UpdatedAt:  r.UpdatedAt.Timestamp,
	}
}

type seriesRowRecord struct {
	updateRecord
}

func (r *seriesRowRecord) fromPartial(o models.SeriesPartial) {
	r.setString("name", o.Name)
	r.setNullString("short_name", o.ShortName)
	r.setString("sport", o.Sport)
	r.setNullString("color", o.Color)
	r.setNullString("data_source", o.DataSource)
	r.setTimestamp("created_at", o.CreatedAt)
	r.setTimestamp("updated_at", o.UpdatedAt)
}

type SeriesStore struct {
	repository
	tableMgr *table
}

func NewSeriesStore() *SeriesStore {
	return &SeriesStore{
		repository: repository{tableName: seriesTable, idColumn: idColumn},
		tableMgr:   seriesTableMgr,
	}
}

func (qb *SeriesStore) table() exp.IdentifierExpression { return qb.tableMgr.table }

func (qb *SeriesStore) selectDataset() *goqu.SelectDataset {
	return dialect.From(qb.table()).Select(qb.table().All())
}

func (qb *SeriesStore) getMany(ctx context.Context, q *goqu.SelectDataset) ([]*models.Series, error) {
	return scanRows(ctx, q, (*seriesRow).resolve)
}

func (qb *SeriesStore) get(ctx context.Context, q *goqu.SelectDataset) (*models.Series, error) {
	ret, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(ret) == 0 {
		return nil, nil
	}
	return ret[0], nil
}

func (qb *SeriesStore) Create(ctx context.Context, newObject *models.Series) error {
	var r seriesRow
	r.fromSeries(*newObject)

	id, err := qb.tableMgr.insertID(ctx, r)
	if err != nil {
		return err
	}

	created, err := qb.Find(ctx, id)
	if err != nil {
		return fmt.Errorf("finding after create: %w", err)
	}

	*newObject = *created
	return nil
}

func (qb *SeriesStore) UpdatePartial(ctx context.Context, id int, partial models.SeriesPartial) (*models.Series, error) {
	r := seriesRowRecord{updateRecord{Record: make(exp.Record)}}
	r.fromPartial(partial)

	if len(r.Record) > 0 {
		if err := qb.tableMgr.updateByID(ctx, id, r.Record); err != nil {
			return nil, err
		}
	}

	return qb.Find(ctx, id)
}

func (qb *SeriesStore) Destroy(ctx context.Context, id int) error {
	return qb.destroyExisting(ctx, []int{id})
}

// Find returns nil, nil if not found.
func (qb *SeriesStore) Find(ctx context.Context, id int) (*models.Series, error) {
	return qb.get(ctx, qb.selectDataset().Where(qb.tableMgr.byID(id)))
}

func (qb *SeriesStore) FindMany(ctx context.Context, ids []int) ([]*models.Series, error) {
	q := qb.selectDataset().Prepared(true).Where(qb.table().Col(idColumn).In(ids))
	unsorted, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}
	return findManyOrdered(ids, unsorted, func(s *models.Series) int { return s.ID }, "series")
}

// FindByName matches the name case-insensitively. Returns nil, nil if not found.
func (qb *SeriesStore) FindByName(ctx context.Context, name string) (*models.Series, error) {
	q := qb.selectDataset().Where(goqu.L("LOWER(series.name) = LOWER(?)", name))
	return qb.get(ctx, q)
}

func (qb *SeriesStore) All(ctx context.Context) ([]*models.Series, error) {
	return qb.getMany(ctx, qb.selectDataset().Order(goqu.L("series.name COLLATE NOCASE").Asc()))
}

// ---------------------------------------------------------------------------
// events

type eventRow struct {
	ID         int         `db:"id" goqu:"skipinsert"`
	SeriesID   int         `db:"series_id"`
	Season     int         `db:"season"`
	Round      null.Int    `db:"round"`
	Name       string      `db:"name"`
	Circuit    zero.String `db:"circuit"`
	Location   zero.String `db:"location"`
	Country    zero.String `db:"country"`
	StartDate  NullDate    `db:"start_date"`
	EndDate    NullDate    `db:"end_date"`
	ExternalID zero.String `db:"external_id"`
	CreatedAt  Timestamp   `db:"created_at"`
	UpdatedAt  Timestamp   `db:"updated_at"`
}

func (r *eventRow) fromEvent(o models.Event) {
	r.ID = o.ID
	r.SeriesID = o.SeriesID
	r.Season = o.Season
	r.Round = intFromPtr(o.Round)
	r.Name = o.Name
	r.Circuit = zero.StringFrom(o.Circuit)
	r.Location = zero.StringFrom(o.Location)
	r.Country = zero.StringFrom(o.Country)
	r.StartDate = NullDateFromDatePtr(o.StartDate)
	r.EndDate = NullDateFromDatePtr(o.EndDate)
	r.ExternalID = zero.StringFrom(o.ExternalID)
	r.CreatedAt = Timestamp{Timestamp: o.CreatedAt}
	r.UpdatedAt = Timestamp{Timestamp: o.UpdatedAt}
}

func (r *eventRow) resolve() *models.Event {
	return &models.Event{
		ID:         r.ID,
		SeriesID:   r.SeriesID,
		Season:     r.Season,
		Round:      nullIntPtr(r.Round),
		Name:       r.Name,
		Circuit:    r.Circuit.String,
		Location:   r.Location.String,
		Country:    r.Country.String,
		StartDate:  r.StartDate.DatePtr(null.Int{}),
		EndDate:    r.EndDate.DatePtr(null.Int{}),
		ExternalID: r.ExternalID.String,
		CreatedAt:  r.CreatedAt.Timestamp,
		UpdatedAt:  r.UpdatedAt.Timestamp,
	}
}

type eventRowRecord struct {
	updateRecord
}

func (r *eventRowRecord) fromPartial(o models.EventPartial) {
	r.setInt("series_id", o.SeriesID)
	r.setInt("season", o.Season)
	r.setNullInt("round", o.Round)
	r.setString("name", o.Name)
	r.setNullString("circuit", o.Circuit)
	r.setNullString("location", o.Location)
	r.setNullString("country", o.Country)
	if o.StartDate.Set {
		r.set("start_date", nullDateValue(o.StartDate))
	}
	if o.EndDate.Set {
		r.set("end_date", nullDateValue(o.EndDate))
	}
	r.setNullString("external_id", o.ExternalID)
	r.setTimestamp("created_at", o.CreatedAt)
	r.setTimestamp("updated_at", o.UpdatedAt)
}

type EventStore struct {
	repository
	tableMgr *table
}

func NewEventStore() *EventStore {
	return &EventStore{
		repository: repository{tableName: eventsTable, idColumn: idColumn},
		tableMgr:   eventsTableMgr,
	}
}

func (qb *EventStore) table() exp.IdentifierExpression { return qb.tableMgr.table }

func (qb *EventStore) selectDataset() *goqu.SelectDataset {
	return dialect.From(qb.table()).Select(qb.table().All())
}

func (qb *EventStore) defaultOrder(q *goqu.SelectDataset) *goqu.SelectDataset {
	return q.Order(
		goqu.I("events.season").Asc(),
		goqu.L("events.round IS NULL").Asc(),
		goqu.I("events.round").Asc(),
		goqu.I("events.start_date").Asc(),
		goqu.I("events.id").Asc(),
	)
}

func (qb *EventStore) getMany(ctx context.Context, q *goqu.SelectDataset) ([]*models.Event, error) {
	return scanRows(ctx, q, (*eventRow).resolve)
}

func (qb *EventStore) get(ctx context.Context, q *goqu.SelectDataset) (*models.Event, error) {
	ret, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(ret) == 0 {
		return nil, nil
	}
	return ret[0], nil
}

func (qb *EventStore) Create(ctx context.Context, newObject *models.Event) error {
	var r eventRow
	r.fromEvent(*newObject)

	id, err := qb.tableMgr.insertID(ctx, r)
	if err != nil {
		return err
	}

	created, err := qb.Find(ctx, id)
	if err != nil {
		return fmt.Errorf("finding after create: %w", err)
	}

	*newObject = *created
	return nil
}

func (qb *EventStore) UpdatePartial(ctx context.Context, id int, partial models.EventPartial) (*models.Event, error) {
	r := eventRowRecord{updateRecord{Record: make(exp.Record)}}
	r.fromPartial(partial)

	if len(r.Record) > 0 {
		if err := qb.tableMgr.updateByID(ctx, id, r.Record); err != nil {
			return nil, err
		}
	}

	return qb.Find(ctx, id)
}

func (qb *EventStore) Destroy(ctx context.Context, id int) error {
	return qb.destroyExisting(ctx, []int{id})
}

// Find returns nil, nil if not found.
func (qb *EventStore) Find(ctx context.Context, id int) (*models.Event, error) {
	return qb.get(ctx, qb.selectDataset().Where(qb.tableMgr.byID(id)))
}

func (qb *EventStore) FindMany(ctx context.Context, ids []int) ([]*models.Event, error) {
	q := qb.selectDataset().Prepared(true).Where(qb.table().Col(idColumn).In(ids))
	unsorted, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}
	return findManyOrdered(ids, unsorted, func(e *models.Event) int { return e.ID }, "event")
}

// FindByExternalID returns nil, nil if not found.
func (qb *EventStore) FindByExternalID(ctx context.Context, seriesID int, externalID string) (*models.Event, error) {
	q := qb.selectDataset().Where(
		qb.table().Col("series_id").Eq(seriesID),
		qb.table().Col("external_id").Eq(externalID),
	)
	return qb.get(ctx, q)
}

func (qb *EventStore) Query(ctx context.Context, options models.EventQueryOptions) ([]*models.Event, error) {
	q := qb.selectDataset()
	if options.SeriesID != nil {
		q = q.Where(qb.table().Col("series_id").Eq(*options.SeriesID))
	}
	if options.Season != nil {
		q = q.Where(qb.table().Col("season").Eq(*options.Season))
	}
	return qb.getMany(ctx, qb.defaultOrder(q))
}

func (qb *EventStore) CountBySeriesID(ctx context.Context, seriesID int) (int, error) {
	q := dialect.Select(goqu.COUNT("*")).From(qb.table()).Where(qb.table().Col("series_id").Eq(seriesID))
	return count(ctx, q)
}

func (qb *EventStore) Seasons(ctx context.Context, seriesID int) ([]int, error) {
	q := dialect.From(qb.table()).Select(qb.table().Col("season")).Distinct().
		Where(qb.table().Col("series_id").Eq(seriesID)).
		Order(qb.table().Col("season").Desc())

	const single = false
	var ret []int
	if err := queryFunc(ctx, q, single, func(r *sqlx.Rows) error {
		var season int
		if err := r.Scan(&season); err != nil {
			return err
		}
		ret = append(ret, season)
		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

// ---------------------------------------------------------------------------
// event sessions

type eventSessionRow struct {
	ID          int           `db:"id" goqu:"skipinsert"`
	EventID     int           `db:"event_id"`
	SessionType string        `db:"session_type"`
	Name        zero.String   `db:"name"`
	StartTime   NullTimestamp `db:"start_time"`
	CreatedAt   Timestamp     `db:"created_at"`
	UpdatedAt   Timestamp     `db:"updated_at"`
}

func (r *eventSessionRow) fromEventSession(o models.EventSession) {
	r.ID = o.ID
	r.EventID = o.EventID
	r.SessionType = o.SessionType.String()
	r.Name = zero.StringFrom(o.Name)
	r.StartTime = NullTimestampFromTimePtr(o.StartTime)
	r.CreatedAt = Timestamp{Timestamp: o.CreatedAt}
	r.UpdatedAt = Timestamp{Timestamp: o.UpdatedAt}
}

func (r *eventSessionRow) resolve() *models.EventSession {
	return &models.EventSession{
		ID:          r.ID,
		EventID:     r.EventID,
		SessionType: models.SessionType(r.SessionType),
		Name:        r.Name.String,
		StartTime:   r.StartTime.TimePtr(),
		CreatedAt:   r.CreatedAt.Timestamp,
		UpdatedAt:   r.UpdatedAt.Timestamp,
	}
}

type eventSessionRowRecord struct {
	updateRecord
}

func (r *eventSessionRowRecord) fromPartial(o models.EventSessionPartial) {
	r.setInt("event_id", o.EventID)
	r.setString("session_type", o.SessionType)
	r.setNullString("name", o.Name)
	r.setNullTimestamp("start_time", o.StartTime)
	r.setTimestamp("created_at", o.CreatedAt)
	r.setTimestamp("updated_at", o.UpdatedAt)
}

type EventSessionStore struct {
	repository
	tableMgr *table
}

func NewEventSessionStore() *EventSessionStore {
	return &EventSessionStore{
		repository: repository{tableName: eventSessionsTable, idColumn: idColumn},
		tableMgr:   eventSessionsTableMgr,
	}
}

func (qb *EventSessionStore) table() exp.IdentifierExpression { return qb.tableMgr.table }

func (qb *EventSessionStore) selectDataset() *goqu.SelectDataset {
	return dialect.From(qb.table()).Select(qb.table().All())
}

func (qb *EventSessionStore) getMany(ctx context.Context, q *goqu.SelectDataset) ([]*models.EventSession, error) {
	return scanRows(ctx, q, (*eventSessionRow).resolve)
}

func (qb *EventSessionStore) get(ctx context.Context, q *goqu.SelectDataset) (*models.EventSession, error) {
	ret, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(ret) == 0 {
		return nil, nil
	}
	return ret[0], nil
}

func (qb *EventSessionStore) Create(ctx context.Context, newObject *models.EventSession) error {
	var r eventSessionRow
	r.fromEventSession(*newObject)

	id, err := qb.tableMgr.insertID(ctx, r)
	if err != nil {
		return err
	}

	created, err := qb.Find(ctx, id)
	if err != nil {
		return fmt.Errorf("finding after create: %w", err)
	}

	*newObject = *created
	return nil
}

func (qb *EventSessionStore) UpdatePartial(ctx context.Context, id int, partial models.EventSessionPartial) (*models.EventSession, error) {
	r := eventSessionRowRecord{updateRecord{Record: make(exp.Record)}}
	r.fromPartial(partial)

	if len(r.Record) > 0 {
		if err := qb.tableMgr.updateByID(ctx, id, r.Record); err != nil {
			return nil, err
		}
	}

	return qb.Find(ctx, id)
}

func (qb *EventSessionStore) Destroy(ctx context.Context, id int) error {
	return qb.destroyExisting(ctx, []int{id})
}

// Find returns nil, nil if not found.
func (qb *EventSessionStore) Find(ctx context.Context, id int) (*models.EventSession, error) {
	return qb.get(ctx, qb.selectDataset().Where(qb.tableMgr.byID(id)))
}

func (qb *EventSessionStore) FindMany(ctx context.Context, ids []int) ([]*models.EventSession, error) {
	q := qb.selectDataset().Prepared(true).Where(qb.table().Col(idColumn).In(ids))
	unsorted, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, err
	}
	return findManyOrdered(ids, unsorted, func(s *models.EventSession) int { return s.ID }, "event session")
}

// sessionTypeOrder sorts sessions without a start time in weekend order.
var sessionTypeOrder = func() string {
	ret := "CASE event_sessions.session_type"
	for i, t := range models.AllSessionType {
		ret += fmt.Sprintf(" WHEN '%s' THEN %d", t, i)
	}
	return ret + fmt.Sprintf(" ELSE %d END", len(models.AllSessionType))
}()

func (qb *EventSessionStore) FindByEventID(ctx context.Context, eventID int) ([]*models.EventSession, error) {
	q := qb.selectDataset().Where(qb.table().Col("event_id").Eq(eventID)).Order(
		goqu.L("event_sessions.start_time IS NULL").Asc(),
		goqu.I("event_sessions.start_time").Asc(),
		goqu.L(sessionTypeOrder).Asc(),
		goqu.I("event_sessions.id").Asc(),
	)
	return qb.getMany(ctx, q)
}
