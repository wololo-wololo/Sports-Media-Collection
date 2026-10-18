import React, { useMemo, useState } from "react";
import { Button, Form } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { Link, useHistory } from "react-router-dom";
import { Helmet } from "react-helmet";
import { faMagnifyingGlass } from "@fortawesome/free-solid-svg-icons";
import * as GQL from "src/core/generated-graphql";
import { Icon } from "src/components/Shared/Icon";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { useTitleProps } from "src/hooks/title";
import { SportsLayout } from "./SportsLayout";
import {
  EmptyState,
  FeedTag,
  LanguageTag,
  Panel,
  SessionBadge,
  SyncButton,
} from "./common";
import {
  defaultSeason,
  formatDuration,
  pickPrimarySeries,
  sessionMessageID,
  shortEventName,
} from "./utils";

const quickFilters = [
  { id: "races", query: "session=RACE" },
  { id: "onboards", query: "feed=ONBOARD" },
  { id: "dvds", query: "source=DVD" },
  { id: "top_rated", query: "rating=80" },
  { id: "incomplete", query: "completeness=PARTIAL" },
];

function recordingTitle(
  r: GQL.RecordingDataFragment,
  intl: ReturnType<typeof useIntl>
) {
  if (r.event) {
    return `${r.event.season} ${shortEventName(r.event.name)}`;
  }
  return (
    r.title ||
    r.files[0]?.basename ||
    intl.formatMessage({ id: "sports.untitled" })
  );
}

const ResumeCard: React.FC<{ r: GQL.RecordingDataFragment }> = ({ r }) => {
  const intl = useIntl();
  const duration = r.files[0]?.duration ?? 0;
  const pct =
    duration > 0 ? Math.min(100, ((r.resume_time ?? 0) / duration) * 100) : 0;
  const left = duration - (r.resume_time ?? 0);
  return (
    <Link className="pw-card" to={`/scenes/${r.id}`}>
      <div className="pw-thumb">
        {r.paths.screenshot && (
          <img src={r.paths.screenshot} alt="" loading="lazy" />
        )}
        <div className="pw-thumb-tags">
          {r.event && (
            <span className="pw-tag pw-tag-solid">
              {r.event.series.short_name || r.event.series.name}
              {r.event.round ? ` · R${r.event.round}` : ""}
            </span>
          )}
          <LanguageTag language={r.language} />
        </div>
        <div className="pw-thumb-title">
          {r.session
            ? intl.formatMessage({
                id: sessionMessageID(r.session.session_type),
              })
            : ""}
        </div>
      </div>
      <div className="pw-progress" aria-hidden="true">
        <div style={{ width: `${pct}%` }} />
      </div>
      <div className="pw-card-body">
        <div className="pw-strong">{recordingTitle(r, intl)}</div>
        <div className="pw-muted pw-small">
          {[
            r.studio?.name,
            left > 0
              ? intl.formatMessage(
                  { id: "sports.time_left" },
                  { time: formatDuration(left) }
                )
              : "",
          ]
            .filter(Boolean)
            .join(" · ")}
        </div>
      </div>
    </Link>
  );
};

const RecentRow: React.FC<{ r: GQL.RecordingDataFragment }> = ({ r }) => {
  const intl = useIntl();
  return (
    <Link className="pw-recent-row" to={`/scenes/${r.id}`}>
      <div className="pw-min0">
        <div className="pw-strong pw-ellipsis">{recordingTitle(r, intl)}</div>
        <div className="pw-muted pw-small pw-ellipsis">
          {r.session
            ? intl.formatMessage({
                id: sessionMessageID(r.session.session_type),
              })
            : r.event
              ? intl.formatMessage({ id: "sports.extra" })
              : r.files[0]?.basename}
        </div>
      </div>
      <div className="pw-ellipsis">{r.studio?.name}</div>
      <div>
        <FeedTag feed={r.feed_type} subject={r.feed_subject} />
      </div>
      <LanguageTag language={r.language} />
      <span className="pw-mono pw-muted pw-right">
        {formatDuration(r.files[0]?.duration)}
      </span>
    </Link>
  );
};

const Gaps: React.FC<{ series: GQL.SeriesDataFragment }> = ({ series }) => {
  const intl = useIntl();
  const season = defaultSeason(series.seasons);
  const { data } = GQL.useSeasonCoverageQuery({
    variables: { series_id: series.id, season },
    skip: !season,
  });

  const gaps = useMemo(() => {
    const now = Date.now();
    const today = new Date().toISOString().slice(0, 10);
    const ret: {
      event: GQL.EventDataFragment;
      session: GQL.EventSessionDataFragment;
      when: string;
    }[] = [];
    for (const ec of data?.seasonCoverage ?? []) {
      for (const sc of ec.sessions) {
        if (sc.recordings > 0) continue;
        const when = sc.session.start_time ?? ec.event.end_date ?? "";
        const past = sc.session.start_time
          ? new Date(sc.session.start_time).getTime() < now
          : !!ec.event.end_date && ec.event.end_date < today;
        if (past) ret.push({ event: ec.event, session: sc.session, when });
      }
    }
    // missing races matter most, then qualifying and sprints, newest first
    const weight = (t: GQL.SessionType) =>
      t === GQL.SessionType.Race
        ? 0
        : t === GQL.SessionType.Qualifying
          ? 1
          : t === GQL.SessionType.Sprint
            ? 2
            : 3;
    return ret
      .sort(
        (a, b) =>
          weight(a.session.session_type) - weight(b.session.session_type) ||
          b.when.localeCompare(a.when)
      )
      .slice(0, 5);
  }, [data]);

  return (
    <Panel
      title={<FormattedMessage id="sports.gaps" />}
      className="pw-list-panel"
    >
      <p className="pw-muted pw-small pw-panel-sub">
        <FormattedMessage id="sports.gaps_description" values={{ season }} />
      </p>
      {gaps.length === 0 && (
        <p className="pw-panel-sub">
          <FormattedMessage id="sports.no_gaps" />
        </p>
      )}
      {gaps.map((g) => (
        <div className="pw-gap" key={g.session.id}>
          <SessionBadge
            type={g.session.session_type}
            className="pw-code-muted"
          />
          <div className="pw-min0 pw-grow">
            <div className="pw-strong pw-ellipsis">
              {g.event.season} {shortEventName(g.event.name)}
            </div>
            <div className="pw-muted pw-small">
              {intl.formatMessage({
                id: sessionMessageID(g.session.session_type),
              })}
            </div>
          </div>
          <Link to={`/events/${g.event.id}`} className="pw-small">
            <FormattedMessage id="sports.open" />
          </Link>
        </div>
      ))}
    </Panel>
  );
};

const Coverage: React.FC<{ series: GQL.SeriesDataFragment }> = ({ series }) => {
  const intl = useIntl();
  const { data } = GQL.useSeriesCoverageQuery({
    variables: { series_id: series.id },
  });
  const seasons = data?.seriesCoverage ?? [];
  const total = seasons.reduce(
    (acc, s) => ({
      sessions: acc.sessions + s.sessions,
      covered: acc.covered + s.covered_sessions,
    }),
    { sessions: 0, covered: 0 }
  );

  const rows = [
    ...seasons.slice(0, 4).map((s) => ({
      key: String(s.season),
      label: String(s.season),
      to: `/series/${series.id}/${s.season}`,
      covered: s.covered_sessions,
      sessions: s.sessions,
    })),
    ...(seasons.length > 4
      ? [
          {
            key: "all",
            label: intl.formatMessage({ id: "sports.all_seasons" }),
            to: `/series/${series.id}`,
            covered: total.covered,
            sessions: total.sessions,
          },
        ]
      : []),
  ];

  return (
    <Panel
      title={
        <FormattedMessage
          id="sports.coverage_of"
          values={{ series: series.name }}
        />
      }
      className="pw-list-panel"
    >
      {rows.map((r) => {
        const pct = r.sessions > 0 ? (r.covered / r.sessions) * 100 : 0;
        return (
          <Link className="pw-stat-row" to={r.to} key={r.key}>
            <div className="pw-stat-head">
              <span className="pw-strong">{r.label}</span>
              <span className="pw-mono pw-muted pw-small">
                <FormattedMessage
                  id="sports.sessions_covered"
                  values={{ covered: r.covered, total: r.sessions }}
                />
              </span>
            </div>
            <div
              className="pw-bar"
              role="meter"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={Math.round(pct)}
              aria-label={r.label}
            >
              <div style={{ width: `${pct}%` }} />
            </div>
          </Link>
        );
      })}
    </Panel>
  );
};

const FirstRun: React.FC = () => (
  <EmptyState title={<FormattedMessage id="sports.welcome_title" />}>
    <ol className="pw-steps">
      <li>
        <p>
          <FormattedMessage id="sports.welcome_sync" />
        </p>
        <SyncButton variant="primary" />
      </li>
      <li>
        <p>
          <FormattedMessage id="sports.welcome_folders" />
        </p>
        <Link to="/settings?tab=library" className="btn btn-secondary">
          <FormattedMessage id="sports.add_folders" />
        </Link>
      </li>
      <li>
        <p>
          <FormattedMessage id="sports.welcome_import" />
        </p>
        <Link to="/import" className="btn btn-secondary">
          <FormattedMessage id="sports.review_import" />
        </Link>
      </li>
    </ol>
  </EmptyState>
);

export const LibraryHome: React.FC = () => {
  const intl = useIntl();
  const history = useHistory();
  const titleProps = useTitleProps({ id: "sports.library" });
  const [search, setSearch] = useState("");

  const { data: seriesData, loading: seriesLoading } = GQL.useAllSeriesQuery();
  const { data: resumeData } = GQL.useFindRecordingsQuery({
    variables: {
      filter: {
        per_page: 4,
        sort: "last_played_at",
        direction: GQL.SortDirectionEnum.Desc,
      },
      scene_filter: {
        resume_time: { value: 0, modifier: GQL.CriterionModifier.GreaterThan },
      },
    },
    fetchPolicy: "cache-and-network",
  });
  const { data: recentData, loading: recentLoading } =
    GQL.useFindRecordingsQuery({
      variables: {
        filter: {
          per_page: 8,
          sort: "created_at",
          direction: GQL.SortDirectionEnum.Desc,
        },
      },
      fetchPolicy: "cache-and-network",
    });

  const series = seriesData?.allSeries ?? [];
  const primary = pickPrimarySeries(series);
  const resume = resumeData?.findScenes.scenes ?? [];
  const recent = recentData?.findScenes.scenes ?? [];

  if (seriesLoading && !seriesData) return <LoadingIndicator />;

  return (
    <SportsLayout activeSeriesID={undefined}>
      <Helmet {...titleProps} />
      <header className="pw-page-head">
        <h1 className="pw-h1">
          <FormattedMessage id="sports.library" />
        </h1>
        <Form
          className="pw-search"
          role="search"
          onSubmit={(e) => {
            e.preventDefault();
            history.push(`/recordings?q=${encodeURIComponent(search.trim())}`);
          }}
        >
          <Icon icon={faMagnifyingGlass} />
          <Form.Control
            type="search"
            value={search}
            aria-label={intl.formatMessage({ id: "sports.search_library" })}
            placeholder={intl.formatMessage({
              id: "sports.search_placeholder",
            })}
            onChange={(e) => setSearch(e.currentTarget.value)}
          />
        </Form>
      </header>

      {series.length === 0 || !primary || primary.event_count === 0 ? (
        <FirstRun />
      ) : (
        <>
          <div className="pw-chips">
            {quickFilters.map((f) => (
              <Link
                key={f.id}
                className="pw-chip"
                to={`/recordings?${f.query}`}
              >
                <FormattedMessage id={`sports.quick.${f.id}`} />
              </Link>
            ))}
          </div>

          {resume.length > 0 && (
            <section className="pw-section">
              <div className="pw-section-head">
                <h2 className="pw-h2">
                  <FormattedMessage id="sports.continue_watching" />
                </h2>
                <span className="pw-muted">
                  <FormattedMessage id="sports.continue_watching_description" />
                </span>
              </div>
              <div className="pw-card-grid">
                {resume.map((r) => (
                  <ResumeCard key={r.id} r={r} />
                ))}
              </div>
            </section>
          )}

          <div className="pw-columns">
            <Panel className="pw-wide pw-list-panel">
              <div className="pw-panel-head">
                <h2 className="pw-h2">
                  <FormattedMessage id="sports.recently_added" />
                </h2>
                <Link to="/recordings?sort=created_at">
                  <FormattedMessage id="sports.see_all" />
                </Link>
              </div>
              {recentLoading && recent.length === 0 && (
                <LoadingIndicator inline small />
              )}
              {!recentLoading && recent.length === 0 && (
                <p className="pw-panel-sub">
                  <FormattedMessage id="sports.no_recordings_yet" />{" "}
                  <Link to="/settings?tab=library">
                    <FormattedMessage id="sports.add_folders" />
                  </Link>
                </p>
              )}
              <div className="pw-scroll-x">
                <div className="pw-recent">
                  {recent.map((r) => (
                    <RecentRow key={r.id} r={r} />
                  ))}
                </div>
              </div>
            </Panel>
            <div className="pw-side">
              <Gaps series={primary} />
              <Coverage series={primary} />
            </div>
          </div>
          <div className="pw-foot-actions">
            <Button
              variant="link"
              onClick={() => history.push(`/series/${primary.id}`)}
            >
              <FormattedMessage id="sports.open_season" />
            </Button>
          </div>
        </>
      )}
    </SportsLayout>
  );
};

export default LibraryHome;
