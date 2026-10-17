package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

// sceneSportsInput holds the sports recording fields shared by the scene
// create, update and bulk update inputs.
type sceneSportsInput struct {
	EventID      *string
	SessionID    *string
	Language     *string
	FeedType     *models.FeedType
	FeedSubject  *string
	Source       *models.RecordingSource
	Completeness *models.Completeness
	Commentators *string
	SyncOffset   *float64
}

func sceneSportsFromCreate(i models.SceneCreateInput) sceneSportsInput {
	return sceneSportsInput{i.EventID, i.SessionID, i.Language, i.FeedType, i.FeedSubject, i.Source, i.Completeness, i.Commentators, i.SyncOffset}
}

func sceneSportsFromUpdate(i models.SceneUpdateInput) sceneSportsInput {
	return sceneSportsInput{i.EventID, i.SessionID, i.Language, i.FeedType, i.FeedSubject, i.Source, i.Completeness, i.Commentators, i.SyncOffset}
}

func sceneSportsFromBulk(i BulkSceneUpdateInput) sceneSportsInput {
	return sceneSportsInput{i.EventID, i.SessionID, i.Language, i.FeedType, i.FeedSubject, i.Source, i.Completeness, i.Commentators, i.SyncOffset}
}

func enumStringPtr[T ~string](v *T) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}

// normaliseLanguage trims and lower-cases a language code so that filters
// match regardless of how it was entered.
func normaliseLanguage(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.ToLower(strings.TrimSpace(*v))
	return &s
}

// applyToNewScene sets the sports fields of a scene being created.
func (in sceneSportsInput) applyToNewScene(s *models.Scene, t changesetTranslator) error {
	var err error
	s.EventID, err = t.intPtrFromString(in.EventID)
	if err != nil {
		return fmt.Errorf("converting event id: %w", err)
	}
	s.SessionID, err = t.intPtrFromString(in.SessionID)
	if err != nil {
		return fmt.Errorf("converting session id: %w", err)
	}

	s.Language = t.string(normaliseLanguage(in.Language))
	s.FeedType = in.FeedType
	s.FeedSubject = t.string(in.FeedSubject)
	s.Source = in.Source
	s.Completeness = in.Completeness
	s.Commentators = t.string(in.Commentators)
	s.SyncOffset = in.SyncOffset
	return nil
}

// applyToPartial sets the sports fields of a scene update.
func (in sceneSportsInput) applyToPartial(p *models.ScenePartial, t changesetTranslator) error {
	var err error
	p.EventID, err = t.optionalIntFromString(in.EventID, "event_id")
	if err != nil {
		return fmt.Errorf("converting event id: %w", err)
	}
	p.SessionID, err = t.optionalIntFromString(in.SessionID, "session_id")
	if err != nil {
		return fmt.Errorf("converting session id: %w", err)
	}

	p.Language = t.optionalString(normaliseLanguage(in.Language), "language")
	p.FeedType = t.optionalString(enumStringPtr(in.FeedType), "feed_type")
	p.FeedSubject = t.optionalString(in.FeedSubject, "feed_subject")
	p.Source = t.optionalString(enumStringPtr(in.Source), "source")
	p.Completeness = t.optionalString(enumStringPtr(in.Completeness), "completeness")
	p.Commentators = t.optionalString(in.Commentators, "commentators")
	p.SyncOffset = t.optionalFloat64(in.SyncOffset, "sync_offset")
	return nil
}

// reconcileSceneEventLinks keeps a recording's event and session consistent:
//   - setting a session also sets the event to the session's event, and fails
//     if a different event was given;
//   - setting or clearing the event clears the current session when it
//     belongs to another event.
//
// eventID and sessionID are the scene's current values (nil for new scenes).
func reconcileSceneEventLinks(ctx context.Context, repo models.Repository, eventID, sessionID *int, p *models.ScenePartial) error {
	if p.SessionID.Set && !p.SessionID.Null {
		session, err := repo.EventSession.Find(ctx, p.SessionID.Value)
		if err != nil {
			return err
		}
		if session == nil {
			return fmt.Errorf("event session with id %d not found", p.SessionID.Value)
		}

		if p.EventID.Set && (p.EventID.Null || p.EventID.Value != session.EventID) {
			return fmt.Errorf("session %d belongs to event %d, not the given event", session.ID, session.EventID)
		}

		p.EventID = models.NewOptionalInt(session.EventID)
		return nil
	}

	if !p.EventID.Set {
		return nil
	}

	if !p.EventID.Null {
		event, err := repo.Event.Find(ctx, p.EventID.Value)
		if err != nil {
			return err
		}
		if event == nil {
			return fmt.Errorf("event with id %d not found", p.EventID.Value)
		}
	}

	if p.SessionID.Set || sessionID == nil {
		return nil
	}

	// the event is changing and no session was given: drop the existing
	// session unless it belongs to the new event
	if p.EventID.Null || eventID == nil || *eventID != p.EventID.Value {
		session, err := repo.EventSession.Find(ctx, *sessionID)
		if err != nil {
			return err
		}
		if session == nil || p.EventID.Null || session.EventID != p.EventID.Value {
			p.SessionID = models.OptionalInt{Set: true, Null: true}
		}
	}

	return nil
}

// reconcileNewSceneEventLinks applies reconcileSceneEventLinks to a scene
// being created.
func reconcileNewSceneEventLinks(ctx context.Context, repo models.Repository, s *models.Scene) error {
	p := models.ScenePartial{
		EventID:   models.NewOptionalIntPtr(s.EventID),
		SessionID: models.NewOptionalIntPtr(s.SessionID),
	}
	if s.EventID == nil {
		p.EventID = models.OptionalInt{}
	}
	if s.SessionID == nil {
		p.SessionID = models.OptionalInt{}
	}

	if err := reconcileSceneEventLinks(ctx, repo, nil, nil, &p); err != nil {
		return err
	}

	if p.EventID.Set && !p.EventID.Null {
		v := p.EventID.Value
		s.EventID = &v
	}
	return nil
}
