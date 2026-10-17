package manager

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stashapp/stash/internal/build"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sports"
	"github.com/stashapp/stash/pkg/sports/jolpica"
)

// SyncCalendarJob syncs series calendars from a calendar source. Seasons are
// fetched outside any transaction and written one season per transaction,
// so a failed or cancelled sync keeps the seasons already done.
type SyncCalendarJob struct {
	Repository models.Repository
	Source     sports.CalendarSource
	Series     sports.SeriesDefaults
	// Seasons to sync; empty syncs every season the source knows.
	Seasons []int
}

func (j *SyncCalendarJob) syncRepository() sports.SyncRepository {
	return sports.SyncRepository{
		Series:       j.Repository.Series,
		Event:        j.Repository.Event,
		EventSession: j.Repository.EventSession,
	}
}

func (j *SyncCalendarJob) Execute(ctx context.Context, progress *job.Progress) error {
	start := time.Now()
	repo := j.syncRepository()

	var series *models.Series
	if err := j.Repository.WithTxn(ctx, func(ctx context.Context) error {
		var err error
		series, err = sports.EnsureSeries(ctx, repo, j.Series)
		return err
	}); err != nil {
		return fmt.Errorf("preparing series %q: %w", j.Series.Name, err)
	}

	seasons := j.Seasons
	if len(seasons) == 0 {
		var err error
		progress.ExecuteTask("Fetching season list", func() {
			seasons, err = j.Source.Seasons(ctx)
		})
		if err != nil {
			return fmt.Errorf("fetching seasons: %w", err)
		}
	}

	progress.SetTotal(len(seasons))
	logger.Infof("Syncing %s calendar for %d season(s)", series.Name, len(seasons))

	var total sports.SyncResult
	var failed []string
	for _, year := range seasons {
		if job.IsCancelled(ctx) {
			logger.Info("Stopping calendar sync due to user request")
			break
		}

		var err error
		progress.ExecuteTask(fmt.Sprintf("Syncing %s %d", series.Name, year), func() {
			err = j.syncSeason(ctx, repo, series.ID, year, &total)
		})
		if err != nil {
			if job.IsCancelled(ctx) || errors.Is(err, context.Canceled) {
				logger.Info("Stopping calendar sync due to user request")
				break
			}
			logger.Errorf("Syncing %s %d: %v", series.Name, year, err)
			failed = append(failed, fmt.Sprint(year))
		}
		progress.Increment()
	}

	logger.Infof("%s calendar sync finished in %s: %s", series.Name, time.Since(start).Round(time.Second), total)

	if len(failed) > 0 {
		return fmt.Errorf("could not sync %d season(s): %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

func (j *SyncCalendarJob) syncSeason(ctx context.Context, repo sports.SyncRepository, seriesID int, year int, total *sports.SyncResult) error {
	events, err := j.Source.Season(ctx, year)
	if err != nil {
		return err
	}

	return j.Repository.WithTxn(ctx, func(ctx context.Context) error {
		r, err := sports.SyncSeason(ctx, repo, seriesID, year, events)
		if err != nil {
			return err
		}
		total.Add(r)
		return nil
	})
}

// calendarUserAgent identifies the app to the calendar API, which requires
// a descriptive user agent.
func calendarUserAgent() string {
	version, _, _ := build.Version()
	if version == "" {
		version = "dev"
	}
	return fmt.Sprintf("SportsMediaCollection/%s (+https://github.com/wololo-wololo/Sports-Media-Collection)", strings.TrimPrefix(version, "v"))
}

// SyncCalendar starts a job syncing the Formula 1 calendar. An empty seasons
// list syncs every season.
func (s *Manager) SyncCalendar(ctx context.Context, seasons []int) int {
	j := &SyncCalendarJob{
		Repository: s.Repository,
		Source:     jolpica.NewClient(config.GetInstance().GetSportsCalendarURL(), calendarUserAgent()),
		Series:     sports.Formula1,
		Seasons:    seasons,
	}

	desc := "Syncing Formula 1 calendar..."
	if len(seasons) == 1 {
		desc = fmt.Sprintf("Syncing Formula 1 %d calendar...", seasons[0])
	}
	return s.JobManager.Add(ctx, desc, j)
}
