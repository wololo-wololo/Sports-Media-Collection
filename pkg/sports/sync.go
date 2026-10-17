package sports

import (
	"context"
	"fmt"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

// SyncRepository is the storage the calendar sync writes to.
type SyncRepository struct {
	Series       models.SeriesReaderWriter
	Event        models.EventReaderWriter
	EventSession models.EventSessionReaderWriter
}

// SeriesDefaults describes a series to create when it does not exist yet.
type SeriesDefaults struct {
	Name       string
	ShortName  string
	Sport      string
	Color      string
	DataSource string
}

// Formula1 is the series synced from Jolpica-F1.
var Formula1 = SeriesDefaults{
	Name:       "Formula 1",
	ShortName:  "F1",
	Sport:      models.DefaultSport,
	Color:      "#E8473C",
	DataSource: "jolpica-f1",
}

// EnsureSeries returns the series named d.Name, creating it if needed. An
// existing series without a data source is linked to d.DataSource; other
// fields the user may have changed are left alone.
func EnsureSeries(ctx context.Context, repo SyncRepository, d SeriesDefaults) (*models.Series, error) {
	existing, err := repo.Series.FindByName(ctx, d.Name)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		if existing.DataSource == "" && d.DataSource != "" {
			p := models.NewSeriesPartial()
			p.DataSource = models.NewOptionalString(d.DataSource)
			return repo.Series.UpdatePartial(ctx, existing.ID, p)
		}
		return existing, nil
	}

	s := models.NewSeries()
	s.Name = d.Name
	s.ShortName = d.ShortName
	if d.Sport != "" {
		s.Sport = d.Sport
	}
	s.Color = d.Color
	s.DataSource = d.DataSource
	if err := repo.Series.Create(ctx, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SyncResult counts the changes made by a sync.
type SyncResult struct {
	EventsCreated   int
	EventsUpdated   int
	SessionsCreated int
	SessionsUpdated int
}

func (r *SyncResult) Add(o SyncResult) {
	r.EventsCreated += o.EventsCreated
	r.EventsUpdated += o.EventsUpdated
	r.SessionsCreated += o.SessionsCreated
	r.SessionsUpdated += o.SessionsUpdated
}

func (r SyncResult) String() string {
	return fmt.Sprintf("%d events created, %d updated; %d sessions created, %d updated",
		r.EventsCreated, r.EventsUpdated, r.SessionsCreated, r.SessionsUpdated)
}

func parseDatePtr(s string) (*models.Date, error) {
	if s == "" {
		return nil, nil
	}
	d, err := models.ParseDate(s)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func roundEqual(a *int, b int) bool {
	return a != nil && *a == b
}

// SyncSeason applies the events of one season to a series. Events are
// matched by external id, or adopted by round when created by hand; sessions
// are matched by type.
//
// Round numbers and external ids follow the source, as do session winners.
// Names, places, dates and start times are only filled in where blank, so
// corrections made in the library survive. Nothing is deleted: events and
// sessions the source no longer lists, and all recordings, are kept.
func SyncSeason(ctx context.Context, repo SyncRepository, seriesID int, season int, events []CalendarEvent) (SyncResult, error) {
	var ret SyncResult

	existing, err := repo.Event.Query(ctx, models.EventQueryOptions{SeriesID: &seriesID, Season: &season})
	if err != nil {
		return ret, err
	}

	byExternalID := make(map[string]*models.Event)
	byRound := make(map[int]*models.Event)
	for _, e := range existing {
		if e.ExternalID != "" {
			byExternalID[e.ExternalID] = e
		}
		if e.Round != nil {
			byRound[*e.Round] = e
		}
	}

	// match every source event before writing, so that renumbered rounds
	// can be freed first without violating the unique round index
	matched := make([]*models.Event, len(events))
	claimed := make(map[int]bool)
	for i, ce := range events {
		if e := byExternalID[ce.ExternalID]; e != nil && !claimed[e.ID] {
			matched[i] = e
			claimed[e.ID] = true
		}
	}
	for i, ce := range events {
		if matched[i] != nil {
			continue
		}
		if e := byRound[ce.Round]; e != nil && e.ExternalID == "" && !claimed[e.ID] {
			matched[i] = e
			claimed[e.ID] = true
		}
	}

	// free rounds held by events that move, and by unmatched events that sit
	// on a round the source now uses for another event
	sourceRounds := make(map[int]bool)
	for _, ce := range events {
		sourceRounds[ce.Round] = true
	}
	free := func(e *models.Event) error {
		p := models.NewEventPartial()
		p.Round = models.OptionalInt{Set: true, Null: true}
		if _, err := repo.Event.UpdatePartial(ctx, e.ID, p); err != nil {
			return err
		}
		e.Round = nil
		return nil
	}
	for i, ce := range events {
		if e := matched[i]; e != nil && e.Round != nil && *e.Round != ce.Round {
			if err := free(e); err != nil {
				return ret, err
			}
		}
	}
	for _, e := range existing {
		if !claimed[e.ID] && e.Round != nil && sourceRounds[*e.Round] {
			if err := free(e); err != nil {
				return ret, err
			}
		}
	}

	for i, ce := range events {
		var eventID int
		e := matched[i]

		start, err := parseDatePtr(ce.StartDate)
		if err != nil {
			return ret, fmt.Errorf("event %q start date: %w", ce.Name, err)
		}
		end, err := parseDatePtr(ce.EndDate)
		if err != nil {
			return ret, fmt.Errorf("event %q end date: %w", ce.Name, err)
		}

		if e == nil {
			n := models.NewEvent()
			n.SeriesID = seriesID
			n.Season = season
			round := ce.Round
			n.Round = &round
			n.Name = ce.Name
			n.Circuit = ce.Circuit
			n.Location = ce.Location
			n.Country = ce.Country
			n.StartDate = start
			n.EndDate = end
			n.ExternalID = ce.ExternalID
			if err := repo.Event.Create(ctx, &n); err != nil {
				return ret, fmt.Errorf("creating event %q: %w", ce.Name, err)
			}
			ret.EventsCreated++
			eventID = n.ID
		} else {
			p := models.NewEventPartial()
			changed := false
			// fill blanks only: names, places and dates may have been
			// corrected or translated by the user
			setStr := func(dst *models.OptionalString, cur, v string) {
				if v != "" && cur == "" {
					*dst = models.NewOptionalString(v)
					changed = true
				}
			}
			if !roundEqual(e.Round, ce.Round) {
				p.Round = models.NewOptionalInt(ce.Round)
				changed = true
			}
			setStr(&p.Name, e.Name, ce.Name)
			setStr(&p.Circuit, e.Circuit, ce.Circuit)
			setStr(&p.Location, e.Location, ce.Location)
			setStr(&p.Country, e.Country, ce.Country)
			if ce.ExternalID != "" && e.ExternalID != ce.ExternalID {
				p.ExternalID = models.NewOptionalString(ce.ExternalID)
				changed = true
			}
			if start != nil && e.StartDate == nil {
				p.StartDate = models.NewOptionalDate(*start)
				changed = true
			}
			if end != nil && e.EndDate == nil {
				p.EndDate = models.NewOptionalDate(*end)
				changed = true
			}

			if changed {
				if _, err := repo.Event.UpdatePartial(ctx, e.ID, p); err != nil {
					return ret, fmt.Errorf("updating event %q: %w", ce.Name, err)
				}
				ret.EventsUpdated++
			}
			eventID = e.ID
		}

		sr, err := syncSessions(ctx, repo, eventID, ce.Sessions)
		if err != nil {
			return ret, fmt.Errorf("event %q: %w", ce.Name, err)
		}
		ret.SessionsCreated += sr.SessionsCreated
		ret.SessionsUpdated += sr.SessionsUpdated
	}

	return ret, nil
}

func syncSessions(ctx context.Context, repo SyncRepository, eventID int, sessions []CalendarSession) (SyncResult, error) {
	var ret SyncResult

	existing, err := repo.EventSession.FindByEventID(ctx, eventID)
	if err != nil {
		return ret, err
	}

	byType := make(map[models.SessionType]*models.EventSession)
	for _, s := range existing {
		if byType[s.SessionType] == nil {
			byType[s.SessionType] = s
		}
	}

	for _, cs := range sessions {
		s := byType[cs.Type]
		if s == nil {
			n := models.NewEventSession()
			n.EventID = eventID
			n.SessionType = cs.Type
			n.Name = cs.Name
			n.StartTime = cs.Start
			n.Winner = cs.Winner
			if err := repo.EventSession.Create(ctx, &n); err != nil {
				return ret, fmt.Errorf("creating %s session: %w", cs.Type, err)
			}
			byType[cs.Type] = &n
			ret.SessionsCreated++
			continue
		}

		p := models.NewEventSessionPartial()
		changed := false
		// the source lists scheduled times; keep times the user corrected
		if cs.Start != nil && s.StartTime == nil {
			p.StartTime = models.NewOptionalTime(*cs.Start)
			changed = true
		}
		// results follow the source, which is corrected after protests
		if cs.Winner != "" && !strings.EqualFold(s.Winner, cs.Winner) {
			p.Winner = models.NewOptionalString(cs.Winner)
			changed = true
		}
		if cs.Name != "" && s.Name == "" {
			p.Name = models.NewOptionalString(cs.Name)
			changed = true
		}

		if changed {
			if _, err := repo.EventSession.UpdatePartial(ctx, s.ID, p); err != nil {
				return ret, fmt.Errorf("updating %s session: %w", cs.Type, err)
			}
			ret.SessionsUpdated++
		}
	}

	return ret, nil
}
