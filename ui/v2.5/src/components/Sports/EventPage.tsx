import React, { useEffect, useMemo, useState } from "react";
import { Button, Form } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { Link, useLocation, useParams } from "react-router-dom";
import { Helmet } from "react-helmet";
import cx from "classnames";
import {
  faChevronLeft,
  faChevronRight,
  faEye,
  faPen,
  faPlay,
  faPlus,
} from "@fortawesome/free-solid-svg-icons";
import * as GQL from "src/core/generated-graphql";
import { Icon } from "src/components/Shared/Icon";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { ErrorMessage } from "src/components/Shared/ErrorMessage";
import { useTitleProps } from "src/hooks/title";
import { SportsLayout } from "./SportsLayout";
import {
  FeedTag,
  LanguageTag,
  Panel,
  SessionBadge,
  SourceTag,
  Stars,
} from "./common";
import { EventDialog, SessionDialog } from "./EventDialog";
import { useSpoilerShield } from "./spoilers";
import {
  completenessMessageID,
  dateRange,
  feedMessageID,
  formatDuration,
  languageName,
  qualityLabel,
  recordingSortKey,
  sessionMessageID,
  sessionSortKey,
  sessionTime,
} from "./utils";

type Recording = GQL.RecordingDataFragment;
type Session = GQL.EventSessionDataFragment;
type GroupBy = "session" | "broadcaster" | "language" | "feed";

const RecordingRow: React.FC<{ r: Recording; showSession?: boolean }> = ({
  r,
  showSession,
}) => {
  const intl = useIntl();
  const file = r.files[0];
  const notes = [
    r.completeness === GQL.Completeness.Partial
      ? intl.formatMessage({
          id: completenessMessageID(GQL.Completeness.Partial),
        })
      : "",
    r.title && r.title !== file?.basename ? r.title : "",
  ].filter(Boolean);

  return (
    <div className="pw-rec">
      <div className="pw-rec-feed">
        {showSession && r.session && (
          <SessionBadge
            type={r.session.session_type}
            className="pw-code-small"
          />
        )}
        {r.feed_type ? (
          <FeedTag feed={r.feed_type} subject={r.feed_subject} />
        ) : (
          <span className="pw-muted pw-small">
            <FormattedMessage id="sports.feed_unknown" />
          </span>
        )}
      </div>
      <div className="pw-min0">
        <div className="pw-strong pw-ellipsis">
          {r.studio?.name ?? (
            <span className="pw-muted">
              <FormattedMessage id="sports.broadcaster_unknown" />
            </span>
          )}
        </div>
        <div className="pw-muted pw-small pw-ellipsis">
          {r.commentators || file?.basename}
        </div>
      </div>
      <LanguageTag language={r.language} />
      <div className="pw-muted pw-small pw-min0 pw-ellipsis">
        <SourceTag source={r.source} /> {notes.join(" · ")}
      </div>
      <span className="pw-mono pw-muted pw-small">
        {formatDuration(file?.duration)}
      </span>
      <span className="pw-mono pw-muted pw-small">{qualityLabel(file)}</span>
      <Stars rating100={r.rating100} />
      <Link className="btn btn-primary pw-play" to={`/scenes/${r.id}`}>
        <Icon icon={faPlay} /> <FormattedMessage id="actions.play" />
      </Link>
    </div>
  );
};

const Result: React.FC<{
  label: React.ReactNode;
  value?: string | null;
  hidden: boolean;
}> = ({ label, value, hidden }) => (
  <div className="pw-kv">
    <span className="pw-muted">{label}</span>
    <span>
      {!value ? (
        "—"
      ) : hidden ? (
        <span className="pw-hidden-result">•••</span>
      ) : (
        value
      )}
    </span>
  </div>
);

export const EventPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const intl = useIntl();
  const { data, loading, error } = GQL.useFindEventQuery({
    variables: { id },
    fetchPolicy: "cache-and-network",
  });
  const event = data?.findEvent;
  const { data: siblings } = GQL.useFindEventsQuery({
    variables: { series_id: event?.series.id, season: event?.season },
    skip: !event,
  });

  const [groupBy, setGroupBy] = useState<GroupBy>("session");
  const [reveal, setReveal] = useState(false);
  const [editingEvent, setEditingEvent] = useState(false);
  const [editingSession, setEditingSession] = useState<Session | "new">();
  const shield = useSpoilerShield();

  const titleProps = useTitleProps(
    event ? `${event.season} ${event.name}` : ""
  );

  const sessions = useMemo(
    () =>
      [...(event?.sessions ?? [])].sort(
        (a, b) => sessionSortKey(a) - sessionSortKey(b)
      ),
    [event]
  );
  const recordings = useMemo(
    () =>
      [...(event?.scenes ?? [])].sort(
        (a, b) => recordingSortKey(a) - recordingSortKey(b)
      ),
    [event]
  );

  // scroll to a session linked from the season grid
  useEffect(() => {
    if (!event || !location.hash) return;
    const el = document.getElementById(location.hash.slice(1));
    el?.scrollIntoView({ block: "start" });
  }, [event, location.hash]);

  if (error) return <ErrorMessage error={error.message} />;
  if (loading && !data) return <LoadingIndicator />;
  if (!event)
    return (
      <ErrorMessage
        error={intl.formatMessage({ id: "sports.event_not_found" })}
      />
    );

  const bySession = new Map<string, Recording[]>();
  const extras: Recording[] = [];
  for (const r of recordings) {
    if (r.session) {
      bySession.set(r.session.id, [...(bySession.get(r.session.id) ?? []), r]);
    } else {
      extras.push(r);
    }
  }

  const race = sessions.find((s) => s.session_type === GQL.SessionType.Race);
  const quali = sessions.find(
    (s) => s.session_type === GQL.SessionType.Qualifying
  );
  const sprint = sessions.find(
    (s) => s.session_type === GQL.SessionType.Sprint
  );
  const raceWatched =
    (race && bySession.get(race.id)?.some((r) => (r.play_count ?? 0) > 0)) ??
    false;
  const hideResults = shield.enabled && !reveal && !raceWatched;

  const languages = new Map<string, number>();
  for (const r of recordings) {
    const k = r.language ?? "";
    languages.set(k, (languages.get(k) ?? 0) + 1);
  }

  const list = siblings?.findEvents ?? [];
  const idx = list.findIndex((e) => e.id === event.id);
  const prev = idx > 0 ? list[idx - 1] : undefined;
  const next = idx >= 0 && idx < list.length - 1 ? list[idx + 1] : undefined;

  function groupKey(r: Recording) {
    switch (groupBy) {
      case "broadcaster":
        return (
          r.studio?.name ??
          intl.formatMessage({ id: "sports.broadcaster_unknown" })
        );
      case "language":
        return r.language
          ? languageName(r.language, intl.locale)
          : intl.formatMessage({ id: "sports.original_audio" });
      case "feed":
        return r.feed_type
          ? intl.formatMessage({ id: feedMessageID(r.feed_type) })
          : intl.formatMessage({ id: "sports.feed_unknown" });
      default:
        return "";
    }
  }

  function renderSessions() {
    return sessions.map((s) => {
      const recs = bySession.get(s.id) ?? [];
      const title =
        s.name || intl.formatMessage({ id: sessionMessageID(s.session_type) });
      const result =
        s.winner &&
        (s.session_type === GQL.SessionType.Qualifying ||
        s.session_type === GQL.SessionType.SprintQualifying
          ? intl.formatMessage({ id: "sports.pole_value" }, { name: s.winner })
          : intl.formatMessage(
              { id: "sports.winner_value" },
              { name: s.winner }
            ));
      return (
        <Panel key={s.id} className="pw-session">
          <div className="pw-session-head" id={`session-${s.id}`}>
            <SessionBadge type={s.session_type} />
            <div className="pw-grow pw-min0">
              <h2 className="pw-h2">{title}</h2>
              <div className="pw-muted pw-small">
                {sessionTime(s.start_time, intl.locale)}
              </div>
            </div>
            {result && (
              <span className="pw-mono pw-muted pw-small">
                {hideResults ? "•••" : result}
              </span>
            )}
            <Button
              variant="link"
              className="pw-icon-btn"
              aria-label={intl.formatMessage({ id: "sports.edit_session" })}
              onClick={() => setEditingSession(s)}
            >
              <Icon icon={faPen} />
            </Button>
          </div>
          <div className="pw-scroll-x">
            <div className="pw-rec-list">
              {recs.map((r) => (
                <RecordingRow key={r.id} r={r} />
              ))}
            </div>
          </div>
          {recs.length === 0 && (
            <div className="pw-session-empty">
              <span className="pw-muted">
                <FormattedMessage id="sports.no_recording_session" />
              </span>
              <Link className="btn btn-secondary" to="/import">
                <FormattedMessage id="sports.find_in_import" />
              </Link>
            </div>
          )}
        </Panel>
      );
    });
  }

  function renderGrouped() {
    const groups = new Map<string, Recording[]>();
    for (const r of recordings) {
      const k = groupKey(r);
      groups.set(k, [...(groups.get(k) ?? []), r]);
    }
    return [...groups.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([k, recs]) => (
        <Panel key={k} className="pw-session">
          <div className="pw-session-head">
            <h2 className="pw-h2 pw-grow">{k}</h2>
            <span className="pw-muted pw-small">
              <FormattedMessage
                id="sports.recording_count"
                values={{ count: recs.length }}
              />
            </span>
          </div>
          <div className="pw-scroll-x">
            <div className="pw-rec-list">
              {recs.map((r) => (
                <RecordingRow key={r.id} r={r} showSession />
              ))}
            </div>
          </div>
        </Panel>
      ));
  }

  return (
    <SportsLayout activeSeriesID={event.series.id}>
      <Helmet {...titleProps} />
      <header className="pw-page-head-stack">
        <nav
          className="pw-crumbs"
          aria-label={intl.formatMessage({ id: "sports.breadcrumbs" })}
        >
          <Link to={`/series/${event.series.id}`}>{event.series.name}</Link> ›{" "}
          <Link to={`/series/${event.series.id}/${event.season}`}>
            {event.season}
          </Link>
          {event.round ? (
            <>
              {" "}
              ›{" "}
              <FormattedMessage
                id="sports.round_n"
                values={{ round: event.round }}
              />
            </>
          ) : null}
        </nav>
        <div className="pw-page-head pw-page-head-end">
          <h1 className="pw-h1 pw-grow">{event.name}</h1>
          <div className="pw-actions">
            {prev && (
              <Link className="btn btn-secondary" to={`/events/${prev.id}`}>
                <Icon icon={faChevronLeft} /> {prev.location || prev.name}
              </Link>
            )}
            {next && (
              <Link className="btn btn-secondary" to={`/events/${next.id}`}>
                {next.location || next.name} <Icon icon={faChevronRight} />
              </Link>
            )}
          </div>
        </div>
        <div className="pw-meta pw-muted">
          {[
            event.circuit,
            event.location !== event.circuit ? event.location : "",
            dateRange(event.start_date, event.end_date, intl.locale),
          ]
            .filter(Boolean)
            .map((m) => (
              <span key={m}>{m}</span>
            ))}
          {sprint && <span className="pw-sprint">SPRINT WEEKEND</span>}
          <span className="pw-tag">
            <FormattedMessage
              id="sports.recording_count"
              values={{ count: recordings.length }}
            />
          </span>
        </div>
      </header>

      <div className="pw-columns">
        <div className="pw-wide pw-stack">
          <div
            className="pw-chips"
            role="group"
            aria-label={intl.formatMessage({ id: "sports.group_by" })}
          >
            <span className="pw-muted">
              <FormattedMessage id="sports.group_by" />
            </span>
            {(["session", "broadcaster", "language", "feed"] as GroupBy[]).map(
              (g) => (
                <button
                  type="button"
                  key={g}
                  className={cx("pw-chip", { on: groupBy === g })}
                  aria-pressed={groupBy === g}
                  onClick={() => setGroupBy(g)}
                >
                  <FormattedMessage id={`sports.group.${g}`} />
                </button>
              )
            )}
            <Button
              variant="secondary"
              className="pw-ml-auto"
              onClick={() => setEditingSession("new")}
            >
              <Icon icon={faPlus} />{" "}
              <FormattedMessage id="sports.add_session" />
            </Button>
          </div>

          {groupBy === "session" ? renderSessions() : renderGrouped()}

          {groupBy === "session" && extras.length > 0 && (
            <Panel className="pw-session">
              <div className="pw-session-head">
                <h2 className="pw-h2 pw-grow">
                  <FormattedMessage id="sports.weekend_extras" />
                </h2>
              </div>
              <div className="pw-scroll-x">
                <div className="pw-rec-list">
                  {extras.map((r) => (
                    <RecordingRow key={r.id} r={r} />
                  ))}
                </div>
              </div>
            </Panel>
          )}

          {sessions.length === 0 && recordings.length === 0 && (
            <Panel>
              <p className="pw-panel-sub">
                <FormattedMessage id="sports.no_sessions_event" />
              </p>
            </Panel>
          )}
        </div>

        <aside className="pw-side">
          <Panel title={<FormattedMessage id="sports.results" />}>
            <div className="pw-panel-sub">
              <Result
                label={<FormattedMessage id="sports.race_winner" />}
                value={race?.winner}
                hidden={hideResults}
              />
              <Result
                label={<FormattedMessage id="sports.pole" />}
                value={quali?.winner}
                hidden={hideResults}
              />
              {sprint && (
                <Result
                  label={<FormattedMessage id="sports.sprint_winner" />}
                  value={sprint.winner}
                  hidden={hideResults}
                />
              )}
              {hideResults && (race?.winner || quali?.winner) && (
                <Button
                  variant="secondary"
                  className="pw-mt"
                  onClick={() => setReveal(true)}
                >
                  <Icon icon={faEye} />{" "}
                  <FormattedMessage id="sports.show_results" />
                </Button>
              )}
              <Form.Check
                type="switch"
                id="spoiler-shield"
                className="pw-mt"
                checked={shield.enabled}
                onChange={(e) => shield.setEnabled(e.currentTarget.checked)}
                label={intl.formatMessage({ id: "sports.spoiler_shield" })}
              />
            </div>
          </Panel>
          {languages.size > 0 && (
            <Panel title={<FormattedMessage id="sports.languages_here" />}>
              <div className="pw-panel-sub pw-tag-list">
                {[...languages.entries()].map(([code, n]) => (
                  <span key={code} className="pw-tag">
                    {code
                      ? code.toUpperCase()
                      : intl.formatMessage({
                          id: "sports.original_audio",
                        })}{" "}
                    · {n}
                  </span>
                ))}
              </div>
            </Panel>
          )}
          <Panel title={<FormattedMessage id="sports.event_details" />}>
            <div className="pw-panel-sub">
              <div className="pw-kv">
                <span className="pw-muted">
                  <FormattedMessage id="sports.circuit" />
                </span>
                <span>{event.circuit || "—"}</span>
              </div>
              <div className="pw-kv">
                <span className="pw-muted">
                  <FormattedMessage id="sports.location" />
                </span>
                <span>
                  {[event.location, event.country].filter(Boolean).join(", ") ||
                    "—"}
                </span>
              </div>
              <div className="pw-kv">
                <span className="pw-muted">
                  <FormattedMessage id="sports.round" />
                </span>
                <span className="pw-mono">{event.round ?? "—"}</span>
              </div>
              <Button
                variant="secondary"
                className="pw-mt"
                onClick={() => setEditingEvent(true)}
              >
                <Icon icon={faPen} />{" "}
                <FormattedMessage id="sports.edit_event" />
              </Button>
            </div>
          </Panel>
        </aside>
      </div>

      {editingEvent && (
        <EventDialog
          seriesID={event.series.id}
          season={event.season}
          event={event}
          onClose={() => setEditingEvent(false)}
        />
      )}
      {editingSession && (
        <SessionDialog
          eventID={event.id}
          session={editingSession === "new" ? undefined : editingSession}
          onClose={() => setEditingSession(undefined)}
        />
      )}
    </SportsLayout>
  );
};

export default EventPage;
