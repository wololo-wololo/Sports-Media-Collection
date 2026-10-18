package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sports/importer"
)

const (
	defaultImportPerPage = 25
	maxImportPerPage     = 200
)

// importContext is a calendar snapshot for matching recordings.
type importContext struct {
	matcher *importer.Matcher
	studios map[int]*models.Studio
}

func (r *Resolver) loadImportContext(ctx context.Context) (*importContext, error) {
	series, err := r.repository.Series.All(ctx)
	if err != nil {
		return nil, err
	}
	events, err := r.repository.Event.Query(ctx, models.EventQueryOptions{})
	if err != nil {
		return nil, err
	}
	sessions, err := r.repository.EventSession.All(ctx)
	if err != nil {
		return nil, err
	}

	studios, err := r.repository.Studio.All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[int]*models.Studio, len(studios))
	broadcasters := make([]importer.Broadcaster, 0, len(studios))
	for _, s := range studios {
		aliases, err := r.repository.Studio.GetAliases(ctx, s.ID)
		if err != nil {
			return nil, err
		}
		byID[s.ID] = s
		broadcasters = append(broadcasters, importer.Broadcaster{ID: s.ID, Name: s.Name, Aliases: aliases})
	}

	rules, err := importer.CompileRules(config.GetInstance().GetSportsFilenamePatterns())
	if err != nil {
		logger.Warnf("Ignoring invalid naming rules: %v", err)
	}

	return &importContext{
		matcher: importer.NewMatcher(series, events, sessions, broadcasters, rules),
		studios: byID,
	}, nil
}

// sceneFile returns the path and modification time of a scene's primary file.
func (r *Resolver) sceneFile(ctx context.Context, s *models.Scene) (string, time.Time, error) {
	if err := s.LoadPrimaryFile(ctx, r.repository.File); err != nil {
		return "", time.Time{}, err
	}
	f := s.Files.Primary()
	if f == nil {
		return "", time.Time{}, nil
	}
	return f.Path, f.ModTime, nil
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (ic *importContext) toGQL(scene *models.Scene, path string, s importer.Suggestion) *ImportSuggestion {
	ret := &ImportSuggestion{
		Scene:           scene,
		Path:            path,
		Confidence:      ImportConfidence(s.Confidence),
		Series:          s.Series,
		Event:           s.Event,
		Session:         s.Session,
		Alternatives:    s.Alternatives,
		BroadcasterName: strPtrOrNil(s.BroadcasterName),
		Language:        strPtrOrNil(s.Language),
		FeedType:        s.FeedType,
		FeedSubject:     strPtrOrNil(s.FeedSubject),
		Source:          s.Source,
		Notes:           s.Notes,
	}
	if ret.Alternatives == nil {
		ret.Alternatives = []*models.Event{}
	}
	if ret.Notes == nil {
		ret.Notes = []string{}
	}
	if s.StudioID != nil {
		ret.Studio = ic.studios[*s.StudioID]
	}
	if s.Part != 0 {
		p := s.Part
		ret.Part = &p
	}
	ret.Fields = make([]*ImportField, 0, len(s.Fields))
	for _, f := range s.Fields {
		ret.Fields = append(ret.Fields, &ImportField{Name: f.Name, Value: f.Value, Guessed: f.Guessed, Origin: f.Origin})
	}
	return ret
}

// dismissedFilter selects which review items to list.
type dismissedFilter int

const (
	openOnly dismissedFilter = iota
	openAndDismissed
	dismissedOnly
)

// reviewScenes returns the recordings to review: the given ids, or the
// recordings without an event, ordered by path.
func (r *Resolver) reviewScenes(ctx context.Context, ids []string, dismissed dismissedFilter, page, perPage int) ([]*models.Scene, int, error) {
	if len(ids) > 0 {
		sceneIDs, err := handleIDList(ids, "scene_ids")
		if err != nil {
			return nil, 0, err
		}
		scenes, err := r.repository.Scene.FindMany(ctx, sceneIDs)
		if err != nil {
			return nil, 0, err
		}
		return scenes, len(scenes), nil
	}

	filter := &models.SceneFilterType{
		Events: &models.MultiCriterionInput{Modifier: models.CriterionModifierIsNull},
	}
	switch dismissed {
	case openOnly:
		f := false
		filter.ImportDismissed = &f
	case dismissedOnly:
		t := true
		filter.ImportDismissed = &t
	}

	sort := "path"
	result, err := r.repository.Scene.Query(ctx, models.SceneQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: &models.FindFilterType{Page: &page, PerPage: &perPage, Sort: &sort},
			Count:      true,
		},
		SceneFilter: filter,
	})
	if err != nil {
		return nil, 0, err
	}
	scenes, err := result.Resolve(ctx)
	if err != nil {
		return nil, 0, err
	}
	return scenes, result.Count, nil
}

func (r *queryResolver) SportsImportSuggestions(ctx context.Context, input *ImportSuggestionsInput) (*FindImportSuggestionsResultType, error) {
	if input == nil {
		input = &ImportSuggestionsInput{}
	}
	page, perPage := 1, defaultImportPerPage
	if input.Page != nil && *input.Page > 0 {
		page = *input.Page
	}
	if input.PerPage != nil && *input.PerPage > 0 {
		perPage = min(*input.PerPage, maxImportPerPage)
	}
	dismissed := openOnly
	if input.IncludeDismissed != nil && *input.IncludeDismissed {
		dismissed = openAndDismissed
	}
	if input.DismissedOnly != nil && *input.DismissedOnly {
		dismissed = dismissedOnly
	}

	ret := &FindImportSuggestionsResultType{Suggestions: []*ImportSuggestion{}}
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ic, err := r.loadImportContext(ctx)
		if err != nil {
			return err
		}

		scenes, count, err := r.reviewScenes(ctx, input.SceneIds, dismissed, page, perPage)
		if err != nil {
			return err
		}
		ret.Count = count

		for _, scene := range scenes {
			path, modTime, err := r.sceneFile(ctx, scene)
			if err != nil {
				return err
			}
			ret.Suggestions = append(ret.Suggestions, ic.toGQL(scene, path, ic.matcher.Match(path, modTime)))
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

// findOrCreateBroadcaster returns the studio for a broadcaster name.
func (r *Resolver) findOrCreateBroadcaster(ctx context.Context, name string) (int, error) {
	existing, err := r.repository.Studio.FindByName(ctx, name, true)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		return existing.ID, nil
	}

	in := models.NewCreateStudioInput()
	in.Name = name
	in.Aliases = models.NewRelatedStrings([]string{})
	in.URLs = models.NewRelatedStrings([]string{})
	in.TagIDs = models.NewRelatedIDs([]int{})
	in.StashIDs = models.NewRelatedStashIDs([]models.StashID{})
	if err := r.repository.Studio.Create(ctx, &in); err != nil {
		return 0, fmt.Errorf("creating broadcaster %q: %w", name, err)
	}
	logger.Infof("Created broadcaster %q", name)
	return in.ID, nil
}

// applySuggestion links a recording to the suggested event and session and
// fills in the recording fields that are still empty.
func (r *Resolver) applySuggestion(ctx context.Context, scene *models.Scene, s importer.Suggestion) (*models.Scene, error) {
	if s.Event == nil {
		return nil, errors.New("no event to link; choose one")
	}

	p := models.NewScenePartial()
	p.EventID = models.NewOptionalInt(s.Event.ID)
	if s.Session != nil {
		p.SessionID = models.NewOptionalInt(s.Session.ID)
	} else {
		p.SessionID = models.OptionalInt{Set: true, Null: true}
	}
	if err := reconcileSceneEventLinks(ctx, r.repository, scene.EventID, scene.SessionID, &p); err != nil {
		return nil, err
	}

	if scene.StudioID == nil {
		switch {
		case s.StudioID != nil:
			p.StudioID = models.NewOptionalInt(*s.StudioID)
		case s.BroadcasterName != "":
			id, err := r.findOrCreateBroadcaster(ctx, s.BroadcasterName)
			if err != nil {
				return nil, err
			}
			p.StudioID = models.NewOptionalInt(id)
		}
	}

	if scene.Language == "" && s.Language != "" {
		p.Language = models.NewOptionalString(strings.ToLower(s.Language))
	}
	if scene.FeedType == nil && s.FeedType != nil {
		p.FeedType = models.NewOptionalString(s.FeedType.String())
	}
	if scene.FeedSubject == "" && s.FeedSubject != "" {
		p.FeedSubject = models.NewOptionalString(s.FeedSubject)
	}
	if scene.Source == nil && s.Source != nil {
		p.Source = models.NewOptionalString(s.Source.String())
	}
	if scene.ImportDismissed {
		p.ImportDismissed = models.NewOptionalBool(false)
	}

	return r.repository.Scene.UpdatePartial(ctx, scene.ID, p)
}

func (r *mutationResolver) SportsImportAccept(ctx context.Context, input ImportAcceptInput) (ret *models.Scene, err error) {
	sceneID, err := parseID(input.SceneID, "scene")
	if err != nil {
		return nil, err
	}
	var eventID int
	if input.EventID != nil {
		if eventID, err = parseID(*input.EventID, "event"); err != nil {
			return nil, err
		}
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		scene, err := r.repository.Scene.Find(ctx, sceneID)
		if err != nil {
			return err
		}
		if scene == nil {
			return fmt.Errorf("scene with id %d not found", sceneID)
		}

		ic, err := r.loadImportContext(ctx)
		if err != nil {
			return err
		}
		path, modTime, err := r.sceneFile(ctx, scene)
		if err != nil {
			return err
		}

		s := ic.matcher.Match(path, modTime)
		if eventID != 0 {
			if s, err = ic.matcher.MatchEvent(path, modTime, eventID); err != nil {
				return err
			}
		}

		_, err = r.applySuggestion(ctx, scene, s)
		return err
	}); err != nil {
		return nil, err
	}

	return r.getScene(ctx, sceneID)
}

func (r *mutationResolver) SportsImportAcceptAll(ctx context.Context, input *ImportAcceptAllInput) (int, error) {
	minConfidence := importer.ConfidenceHigh
	var ids []string
	if input != nil {
		if input.MinConfidence != nil {
			minConfidence = importer.Confidence(*input.MinConfidence)
		}
		ids = input.SceneIds
	}
	if minConfidence == importer.ConfidenceNone {
		return 0, errors.New("recordings without an event cannot be accepted")
	}

	accepted := 0
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		ic, err := r.loadImportContext(ctx)
		if err != nil {
			return err
		}

		scenes, _, err := r.reviewScenes(ctx, ids, openOnly, 1, models.PerPageAll)
		if err != nil {
			return err
		}

		for _, scene := range scenes {
			if scene.EventID != nil || scene.ImportDismissed {
				continue
			}
			path, modTime, err := r.sceneFile(ctx, scene)
			if err != nil {
				return err
			}
			s := ic.matcher.Match(path, modTime)
			if s.Event == nil || !s.Confidence.AtLeast(minConfidence) {
				continue
			}
			if _, err := r.applySuggestion(ctx, scene, s); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			accepted++
		}
		return nil
	}); err != nil {
		return 0, err
	}

	logger.Infof("Import: accepted %d suggestion(s)", accepted)
	return accepted, nil
}

func (r *mutationResolver) SportsImportDismiss(ctx context.Context, input ImportDismissInput) (bool, error) {
	ids, err := handleIDList(input.SceneIds, "scene_ids")
	if err != nil {
		return false, err
	}

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		for _, id := range ids {
			p := models.NewScenePartial()
			p.ImportDismissed = models.NewOptionalBool(input.Dismissed)
			if _, err := r.repository.Scene.UpdatePartial(ctx, id, p); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return false, err
	}

	return true, nil
}
