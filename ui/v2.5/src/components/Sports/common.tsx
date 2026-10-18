import React, { useCallback, useState } from "react";
import { Button, ProgressBar } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import cx from "classnames";
import { faRotate } from "@fortawesome/free-solid-svg-icons";
import * as GQL from "src/core/generated-graphql";
import { Icon } from "src/components/Shared/Icon";
import { useToast } from "src/hooks/Toast";
import { JobFragment, useMonitorJob } from "src/utils/job";
import {
  feedMessageID,
  languageName,
  languageTag,
  sessionCode,
  sourceMessageID,
} from "./utils";

export const FeedTag: React.FC<{
  feed?: GQL.FeedType | null;
  subject?: string | null;
}> = ({ feed, subject }) => {
  const intl = useIntl();
  if (!feed) return null;
  const kind =
    feed === GQL.FeedType.Onboard
      ? "onboard"
      : feed === GQL.FeedType.WorldFeed
        ? "world"
        : "data";
  return (
    <span className={cx("pw-tag", `pw-tag-${kind}`)}>
      {intl.formatMessage({ id: feedMessageID(feed) })}
      {subject ? ` · ${subject}` : ""}
    </span>
  );
};

export const LanguageTag: React.FC<{ language?: string | null }> = ({
  language,
}) => {
  const intl = useIntl();
  const title = language
    ? languageName(language, intl.locale)
    : intl.formatMessage({ id: "sports.original_audio" });
  return (
    <span className="pw-tag" title={title}>
      {languageTag(language)}
    </span>
  );
};

export const SourceTag: React.FC<{ source?: GQL.RecordingSource | null }> = ({
  source,
}) => {
  const intl = useIntl();
  if (!source) return null;
  return (
    <span
      className={cx("pw-tag", {
        "pw-tag-dvd": source === GQL.RecordingSource.Dvd,
      })}
    >
      {intl.formatMessage({ id: sourceMessageID(source) })}
    </span>
  );
};

export const SessionBadge: React.FC<{
  type: GQL.SessionType;
  className?: string;
}> = ({ type, className }) => (
  <span className={cx("pw-code", className)}>{sessionCode(type)}</span>
);

export const SeriesDot: React.FC<{ color?: string | null }> = ({ color }) => (
  <span
    className="pw-dot"
    style={{ background: color || "#8c96a3" }}
    aria-hidden="true"
  />
);

// rating100 in steps of 20 shown as five stars
export const Stars: React.FC<{ rating100?: number | null }> = ({
  rating100,
}) => {
  const intl = useIntl();
  if (!rating100) return <span className="pw-muted">—</span>;
  const n = Math.max(0, Math.min(5, Math.round(rating100 / 20)));
  return (
    <span
      className="pw-stars"
      role="img"
      aria-label={intl.formatMessage({ id: "sports.stars" }, { count: n })}
    >
      {"★".repeat(n)}
      <span>{"★".repeat(5 - n)}</span>
    </span>
  );
};

export const Panel: React.FC<
  React.PropsWithChildren<{ className?: string; title?: React.ReactNode }>
> = ({ className, title, children }) => (
  <section className={cx("pw-panel", className)}>
    {title && <h2 className="pw-h2 pw-panel-title">{title}</h2>}
    {children}
  </section>
);

export const EmptyState: React.FC<
  React.PropsWithChildren<{ title: React.ReactNode }>
> = ({ title, children }) => (
  <div className="pw-empty">
    <h2 className="pw-h2">{title}</h2>
    {children}
  </div>
);

// Starts a calendar sync job and reports its progress.
export function useCalendarSync(onDone?: () => void) {
  const Toast = useToast();
  const intl = useIntl();
  const [syncCalendar] = GQL.useSyncSeriesCalendarMutation();
  const [jobID, setJobID] = useState<string>();

  const onComplete = useCallback(
    (job?: JobFragment) => {
      setJobID(undefined);
      if (job?.status === GQL.JobStatus.Failed) {
        Toast.error(
          job.error ?? intl.formatMessage({ id: "sports.sync_failed" })
        );
      } else if (job?.status !== GQL.JobStatus.Cancelled) {
        Toast.success(intl.formatMessage({ id: "sports.sync_done" }));
      }
      onDone?.();
    },
    [Toast, intl, onDone]
  );

  const { job } = useMonitorJob(jobID, onComplete);

  async function start(seasons?: number[]) {
    try {
      const r = await syncCalendar({ variables: { input: { seasons } } });
      setJobID(r.data?.syncSeriesCalendar);
    } catch (e) {
      Toast.error(e);
    }
  }

  return { start, running: !!jobID, job };
}

export const SyncButton: React.FC<{
  seasons?: number[];
  onDone?: () => void;
  variant?: string;
  label?: React.ReactNode;
}> = ({ seasons, onDone, variant = "secondary", label }) => {
  const { start, running, job } = useCalendarSync(onDone);
  return (
    <div className="pw-sync">
      <Button
        variant={variant}
        disabled={running}
        onClick={() => start(seasons)}
      >
        <Icon icon={faRotate} className={cx({ "fa-spin": running })} />{" "}
        {label ?? <FormattedMessage id="sports.sync_calendar" />}
      </Button>
      {running && (
        <div className="pw-sync-progress">
          <ProgressBar
            now={(job?.progress ?? 0) * 100}
            variant="warning"
            aria-label="progress"
          />
          <span className="pw-muted">
            {job?.subTasks?.[0] ?? job?.description}
          </span>
        </div>
      )}
    </div>
  );
};
