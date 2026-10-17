package sqlite

import (
	"context"
	"fmt"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

// sportsEventJoin left-joins the event of a recording as "events".
func sportsEventJoin(f *filterBuilder, _ joinType) {
	f.addLeftJoin(eventsTable, "", "events.id = scenes.event_id")
}

// sportsSessionJoin left-joins the session of a recording as "event_sessions".
func sportsSessionJoin(f *filterBuilder, _ joinType) {
	f.addLeftJoin(eventSessionsTable, "", "event_sessions.id = scenes.session_id")
}

// idListCriterionHandler filters a nullable foreign key column by a list of ids.
// INCLUDES / EQUALS match any of the ids, EXCLUDES / NOT_EQUALS match rows
// without any of them (including rows where the column is null).
func idListCriterionHandler(c *models.MultiCriterionInput, column string) criterionHandlerFunc {
	return func(ctx context.Context, f *filterBuilder) {
		if c == nil {
			return
		}

		ids := make([]interface{}, 0, len(c.Value))
		for _, v := range c.Value {
			id, err := strconv.Atoi(v)
			if err != nil {
				f.setError(fmt.Errorf("invalid id %q: %w", v, err))
				return
			}
			ids = append(ids, id)
		}

		excludes := make([]interface{}, 0, len(c.Excludes))
		for _, v := range c.Excludes {
			id, err := strconv.Atoi(v)
			if err != nil {
				f.setError(fmt.Errorf("invalid id %q: %w", v, err))
				return
			}
			excludes = append(excludes, id)
		}

		switch c.Modifier {
		case models.CriterionModifierIsNull:
			f.addWhere(column + " IS NULL")
		case models.CriterionModifierNotNull:
			f.addWhere(column + " IS NOT NULL")
		case models.CriterionModifierExcludes, models.CriterionModifierNotEquals:
			if len(ids) > 0 {
				f.addWhere(fmt.Sprintf("(%s IS NULL OR %s NOT IN %s)", column, column, getInBinding(len(ids))), ids...)
			}
		default:
			if len(ids) > 0 {
				f.addWhere(fmt.Sprintf("%s IN %s", column, getInBinding(len(ids))), ids...)
			}
		}

		if len(excludes) > 0 {
			f.addWhere(fmt.Sprintf("(%s IS NULL OR %s NOT IN %s)", column, column, getInBinding(len(excludes))), excludes...)
		}
	}
}

// enumListCriterionHandler filters a nullable text column by a list of enum
// values. EXCLUDES keeps rows where the column is unset.
func enumListCriterionHandler[T ~string](modifier models.CriterionModifier, values []T, column string, addJoinFn func(f *filterBuilder, joinType joinType)) criterionHandlerFunc {
	return func(ctx context.Context, f *filterBuilder) {
		if !modifier.IsValid() {
			return
		}

		args := make([]interface{}, len(values))
		for i, v := range values {
			args[i] = string(v)
		}

		if addJoinFn != nil {
			addJoinFn(f, joinTypeLeft)
		}

		switch modifier {
		case models.CriterionModifierIsNull:
			f.addWhere(fmt.Sprintf("(%s IS NULL OR %s = '')", column, column))
		case models.CriterionModifierNotNull:
			f.addWhere(fmt.Sprintf("(%s IS NOT NULL AND %s != '')", column, column))
		case models.CriterionModifierExcludes, models.CriterionModifierNotEquals:
			if len(args) > 0 {
				f.addWhere(fmt.Sprintf("(%s IS NULL OR %s NOT IN %s)", column, column, getInBinding(len(args))), args...)
			}
		case models.CriterionModifierIncludes, models.CriterionModifierEquals:
			if len(args) > 0 {
				f.addWhere(fmt.Sprintf("%s IN %s", column, getInBinding(len(args))), args...)
			}
		default:
			f.setError(fmt.Errorf("unsupported modifier %s for %s", modifier, column))
		}
	}
}

// seriesCriterionHandler filters recordings by the series of their event.
func seriesCriterionHandler(c *models.MultiCriterionInput) criterionHandlerFunc {
	return func(ctx context.Context, f *filterBuilder) {
		if c == nil {
			return
		}
		sportsEventJoin(f, joinTypeLeft)
		idListCriterionHandler(c, "events.series_id")(ctx, f)
	}
}

func sceneSportsCriterionHandlers(sceneFilter *models.SceneFilterType) compoundHandler {
	ret := compoundHandler{
		seriesCriterionHandler(sceneFilter.Series),
		idListCriterionHandler(sceneFilter.Events, "scenes.event_id"),
		idListCriterionHandler(sceneFilter.Sessions, "scenes.session_id"),
		criterionHandlerFunc(func(ctx context.Context, f *filterBuilder) {
			if sceneFilter.Season != nil {
				sportsEventJoin(f, joinTypeLeft)
				intCriterionHandler(sceneFilter.Season, "events.season", nil)(ctx, f)
			}
		}),
		stringCriterionHandler(sceneFilter.Language, "scenes.language"),
		stringCriterionHandler(sceneFilter.FeedSubject, "scenes.feed_subject"),
		stringCriterionHandler(sceneFilter.Commentators, "scenes.commentators"),
		boolCriterionHandler(sceneFilter.ImportDismissed, "scenes.import_dismissed", nil),
	}

	if c := sceneFilter.SessionType; c != nil {
		ret = append(ret, enumListCriterionHandler(c.Modifier, c.Value, "event_sessions.session_type", sportsSessionJoin))
	}
	if c := sceneFilter.FeedType; c != nil {
		ret = append(ret, enumListCriterionHandler(c.Modifier, c.Value, "scenes.feed_type", nil))
	}
	if c := sceneFilter.Source; c != nil {
		ret = append(ret, enumListCriterionHandler(c.Modifier, c.Value, "scenes.source", nil))
	}
	if c := sceneFilter.Completeness; c != nil {
		ret = append(ret, enumListCriterionHandler(c.Modifier, c.Value, "scenes.completeness", nil))
	}

	return ret
}
