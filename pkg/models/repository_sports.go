package models

import "context"

// SeriesReaderWriter provides all series methods.
type SeriesReaderWriter interface {
	Find(ctx context.Context, id int) (*Series, error)
	FindMany(ctx context.Context, ids []int) ([]*Series, error)
	FindByName(ctx context.Context, name string) (*Series, error)
	All(ctx context.Context) ([]*Series, error)

	Create(ctx context.Context, newSeries *Series) error
	UpdatePartial(ctx context.Context, id int, partial SeriesPartial) (*Series, error)
	Destroy(ctx context.Context, id int) error
}

// EventQueryOptions narrows an event lookup. Unset fields are ignored.
type EventQueryOptions struct {
	SeriesID *int
	Season   *int
}

// EventReaderWriter provides all event methods.
type EventReaderWriter interface {
	Find(ctx context.Context, id int) (*Event, error)
	FindMany(ctx context.Context, ids []int) ([]*Event, error)
	// FindByExternalID returns the event of a series with the given data
	// source identifier, or nil if there is none.
	FindByExternalID(ctx context.Context, seriesID int, externalID string) (*Event, error)
	// Query returns events ordered by season, then round, then start date.
	Query(ctx context.Context, options EventQueryOptions) ([]*Event, error)
	CountBySeriesID(ctx context.Context, seriesID int) (int, error)
	// Seasons returns the distinct seasons of a series in descending order.
	Seasons(ctx context.Context, seriesID int) ([]int, error)

	Create(ctx context.Context, newEvent *Event) error
	UpdatePartial(ctx context.Context, id int, partial EventPartial) (*Event, error)
	Destroy(ctx context.Context, id int) error
}

// EventSessionReaderWriter provides all event session methods.
type EventSessionReaderWriter interface {
	Find(ctx context.Context, id int) (*EventSession, error)
	FindMany(ctx context.Context, ids []int) ([]*EventSession, error)
	// FindByEventID returns the sessions of an event in chronological order.
	FindByEventID(ctx context.Context, eventID int) ([]*EventSession, error)

	Create(ctx context.Context, newSession *EventSession) error
	UpdatePartial(ctx context.Context, id int, partial EventSessionPartial) (*EventSession, error)
	Destroy(ctx context.Context, id int) error
}
