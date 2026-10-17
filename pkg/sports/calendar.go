// Package sports syncs series calendars (events, sessions and results) from
// external data sources into the library.
package sports

import (
	"context"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

// CalendarSession is a session as reported by a calendar source.
type CalendarSession struct {
	Type models.SessionType
	// Name overrides the display label, e.g. "Sprint Shootout". Optional.
	Name string
	// Date is the session day as YYYY-MM-DD. Optional.
	Date string
	// Start is the scheduled start time, when the source knows it.
	Start *time.Time
	// Winner is the winner of the session (pole sitter for qualifying).
	Winner string
}

// CalendarEvent is an event as reported by a calendar source.
type CalendarEvent struct {
	Season int
	Round  int
	Name   string
	// ExternalID identifies the event at the source. It must stay the same
	// when the event is renumbered within its season.
	ExternalID string
	Circuit    string
	Location   string
	Country    string
	// StartDate and EndDate are YYYY-MM-DD.
	StartDate string
	EndDate   string
	Sessions  []CalendarSession
}

// CalendarSource provides calendars for one series.
type CalendarSource interface {
	// Seasons returns all seasons the source has data for, oldest first.
	Seasons(ctx context.Context) ([]int, error)
	// Season returns the events of a season in round order.
	Season(ctx context.Context, year int) ([]CalendarEvent, error)
}
