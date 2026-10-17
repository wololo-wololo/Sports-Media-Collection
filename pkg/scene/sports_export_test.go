package scene

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/jsonschema"
	"github.com/stashapp/stash/pkg/models/mocks"
)

func TestSportsFieldsRoundTrip(t *testing.T) {
	db := mocks.NewDatabase()
	db.Scene.On("GetCover", testCtx, mock.Anything).Return(nil, nil)
	db.Scene.On("GetViewDates", testCtx, mock.Anything).Return(nil, nil)
	db.Scene.On("GetODates", testCtx, mock.Anything).Return(nil, nil)
	db.Scene.On("GetCustomFields", testCtx, mock.Anything).Return(map[string]interface{}{}, nil)

	feed := models.FeedTypeOnboard
	source := models.RecordingSourceDVD
	complete := models.CompletenessPartial
	offset := 0.0
	in := createEmptyScene(sceneID)
	in.Title = "Onboard"
	in.Language = "de"
	in.FeedType = &feed
	in.FeedSubject = "Michael Schumacher"
	in.Source = &source
	in.Completeness = &complete
	in.Commentators = "Heiko Wasser"
	in.SyncOffset = &offset

	out, err := ToBasicJSON(testCtx, db.Scene, &in)
	require.NoError(t, err)
	assert.Equal(t, "ONBOARD", out.FeedType)
	assert.Equal(t, "DVD", out.Source)
	assert.Equal(t, "PARTIAL", out.Completeness)
	require.NotNil(t, out.SyncOffset)

	i := Importer{}
	back := i.sceneJSONToScene(*out)
	assert.Equal(t, in.Language, back.Language)
	assert.Equal(t, *in.FeedType, *back.FeedType)
	assert.Equal(t, in.FeedSubject, back.FeedSubject)
	assert.Equal(t, *in.Source, *back.Source)
	assert.Equal(t, *in.Completeness, *back.Completeness)
	assert.Equal(t, in.Commentators, back.Commentators)
	assert.Equal(t, 0.0, *back.SyncOffset)

	bad := i.sceneJSONToScene(jsonschema.Scene{Title: "x", FeedType: "SATELLITE", Source: ""})
	assert.Nil(t, bad.FeedType, "unknown enum values are dropped")
	assert.Nil(t, bad.Source)
}
