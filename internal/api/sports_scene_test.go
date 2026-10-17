package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stashapp/stash/pkg/models"
)

type fakeEvents struct {
	models.EventReaderWriter
	byID map[int]*models.Event
}

func (f fakeEvents) Find(_ context.Context, id int) (*models.Event, error) {
	return f.byID[id], nil
}

type fakeSessions struct {
	models.EventSessionReaderWriter
	byID map[int]*models.EventSession
}

func (f fakeSessions) Find(_ context.Context, id int) (*models.EventSession, error) {
	return f.byID[id], nil
}

func TestReconcileSceneEventLinks(t *testing.T) {
	const (
		brazil = 1
		suzuka = 2
		race   = 10 // Brazil race
		quali  = 11 // Brazil qualifying
		suzR   = 20 // Suzuka race
	)

	repo := models.Repository{
		Event: fakeEvents{byID: map[int]*models.Event{
			brazil: {ID: brazil}, suzuka: {ID: suzuka},
		}},
		EventSession: fakeSessions{byID: map[int]*models.EventSession{
			race:  {ID: race, EventID: brazil},
			quali: {ID: quali, EventID: brazil},
			suzR:  {ID: suzR, EventID: suzuka},
		}},
	}

	set := models.NewOptionalInt
	null := models.OptionalInt{Set: true, Null: true}
	unset := models.OptionalInt{}
	ptr := func(v int) *int { return &v }

	tests := []struct {
		name               string
		curEvent, curSess  *int
		event, session     models.OptionalInt
		wantEvent, wantSes models.OptionalInt
		wantErr            bool
	}{
		{"nothing set", ptr(brazil), ptr(race), unset, unset, unset, unset, false},
		{"session sets its event", nil, nil, unset, set(race), set(brazil), set(race), false},
		{"session with matching event", nil, nil, set(brazil), set(race), set(brazil), set(race), false},
		{"session with other event fails", nil, nil, set(suzuka), set(race), unset, unset, true},
		{"session with null event fails", ptr(brazil), nil, null, set(race), unset, unset, true},
		{"unknown session fails", nil, nil, unset, set(99), unset, unset, true},
		{"unknown event fails", nil, nil, set(99), unset, unset, unset, true},
		{"new event drops session of old event", ptr(brazil), ptr(race), set(suzuka), unset, set(suzuka), null, false},
		{"same event keeps session", ptr(brazil), ptr(race), set(brazil), unset, set(brazil), unset, false},
		{"clearing event drops session", ptr(brazil), ptr(race), null, unset, null, null, false},
		{"explicit session change is kept", ptr(brazil), ptr(race), unset, set(quali), set(brazil), set(quali), false},
		{"clearing session keeps event", ptr(brazil), ptr(race), unset, null, unset, null, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := models.ScenePartial{EventID: tt.event, SessionID: tt.session}
			err := reconcileSceneEventLinks(context.Background(), repo, tt.curEvent, tt.curSess, &p)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantEvent, p.EventID, "event")
			assert.Equal(t, tt.wantSes, p.SessionID, "session")
		})
	}

	t.Run("new scene with only a session gets its event", func(t *testing.T) {
		s := models.Scene{SessionID: ptr(suzR)}
		require.NoError(t, reconcileNewSceneEventLinks(context.Background(), repo, &s))
		require.NotNil(t, s.EventID)
		assert.Equal(t, suzuka, *s.EventID)
	})

	t.Run("new scene with neither is untouched", func(t *testing.T) {
		s := models.Scene{}
		require.NoError(t, reconcileNewSceneEventLinks(context.Background(), repo, &s))
		assert.Nil(t, s.EventID)
		assert.Nil(t, s.SessionID)
	})
}

func TestNormaliseLanguage(t *testing.T) {
	assert.Nil(t, normaliseLanguage(nil))
	v := "  EN-gb "
	assert.Equal(t, "en-gb", *normaliseLanguage(&v))
}
