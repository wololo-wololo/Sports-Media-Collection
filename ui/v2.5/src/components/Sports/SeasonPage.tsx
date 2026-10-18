import React, { useMemo, useState } from "react";
import { Button, Form } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { Link, Redirect, useHistory, useParams } from "react-router-dom";
import { Helmet } from "react-helmet";
import cx from "classnames";
import {
  faDownload,
  faEye,
  faPen,
  faPlus,
} from "@fortawesome/free-solid-svg-icons";
import * as GQL from "src/core/generated-graphql";
import { Icon } from "src/components/Shared/Icon";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { ErrorMessage } from "src/components/Shared/ErrorMessage";
import { useTitleProps } from "src/hooks/title";
import { SportsLayout } from "./SportsLayout";
import { EmptyState, Panel, SyncButton } from "./common";
import { EventDialog } from "./EventDialog";
import { SeriesDialog } from "./SeriesDialog";
import { useSpoilerShield } from "./spoilers";
import {
  defaultSeason,
  pickPrimarySeries,
  sessionCode,
  sessionMessageID,
  sessionSortKey,
  shortDate,
} from "./utils";

type Coverage = GQL.SeasonCoverageQuery["seasonCoverage"][number];
type SessionCov = Coverage["sessions"][number];

const ST = GQL.SessionType;

// Five columns fit every Formula 1 weekend format: the second and third
// slots hold either practice or the sprint sessions.
const slotOf: Partial<Record<GQL.SessionType, number>> = {
  [ST.Practice_1]: 0,
  [ST.Practice]: 0,
  [ST.Practice_2]: 1,
  [ST.SprintQualifying]: 1,
  [ST.Practice_3]: 2,
  [ST.Sprint]: 2,
  [ST.Qualifying]: 3,
  [ST.Race]: 4,
};

function slotLayout(events: Coverage[]) {
  const ret: (SessionCov | undefined)[][] = [];
  for (const e of events) {
    const slots: (SessionCov | undefined)[] = [
      undefined,
      undefined,
      undefined,
      undefined,
      undefined,
    ];
    for (const s of e.sessions) {
      const i = slotOf[s.session.session_type];
      if (i === undefined || slots[i]) return undefined;
      slots[i] = s;
    }
    ret.push(slots);
  }
  return ret;
}

const Cell: React.FC<{ eventID: string; s?: SessionCov }> = ({
  eventID,
  s,
}) => {
  const intl = useIntl();
  if (!s) return <span className="pw-cell pw-cell-na" aria-hidden="true" />;

  const code = sessionCode(s.session.session_type);
  const name = intl.formatMessage({
    id: sessionMessageID(s.session.session_type),
  });
  let cls = "pw-cell-miss";
  let mark = "+";
  let label = intl.formatMessage(
    { id: "sports.cell_missing" },
    { session: name }
  );
  if (s.recordings > 0 && s.partial === s.recordings) {
    cls = "pw-cell-part";
    mark = "½";
    label = intl.formatMessage(
      { id: "sports.cell_partial" },
      { session: name }
    );
  } else if (s.recordings > 1) {
    cls = "pw-cell-multi";
    mark = String(s.recordings);
    label = intl.formatMessage(
      { id: "sports.cell_count" },
      { session: name, count: s.recordings }
    );
  } else if (s.recordings === 1) {
    cls = "pw-cell-have";
    mark = "1";
    label = intl.formatMessage(
      { id: "sports.cell_count" },
      { session: name, count: 1 }
    );
  }

  return (
    <Link
      to={`/events/${eventID}#session-${s.session.id}`}
      className={cx("pw-cell", cls)}
      title={label}
      aria-label={label}
    >
      <span>{code}</span>
      <span>{mark}</span>
    </Link>
  );
};

function missingCSV(events: Coverage[], intl: ReturnType<typeof useIntl>) {
  const rows = [["season", "round", "event", "session", "start"]];
  for (const e of events) {
    for (const s of e.sessions) {
      if (s.recordings > 0) continue;
      rows.push([
        String(e.event.season),
        String(e.event.round ?? ""),
        e.event.name,
        intl.formatMessage({ id: sessionMessageID(s.session.session_type) }),
        s.session.start_time ?? e.event.end_date ?? "",
      ]);
    }
  }
  return rows
    .map((r) => r.map((c) => `"${c.replace(/"/g, '""')}"`).join(","))
    .join("\r\n");
}

const SeasonGrid: React.FC<{
  series: GQL.SeriesDataFragment;
  season: number;
}> = ({ series, season }) => {
  const intl = useIntl();
  const { enabled: shield } = useSpoilerShield();
  const [showResults, setShowResults] = useState(false);
  const [gapsOnly, setGapsOnly] = useState(false);
  const [editing, setEditing] = useState<GQL.EventDataFragment | "new">();

  const { data, loading, error, refetch } = GQL.useSeasonCoverageQuery({
    variables: { series_id: series.id, season },
    fetchPolicy: "cache-and-network",
  });

  const events = useMemo(() => {
    const list = (data?.seasonCoverage ?? []).map((e) => ({
      ...e,
      sessions: [...e.sessions].sort(
        (a, b) => sessionSortKey(a.session) - sessionSortKey(b.session)
      ),
    }));
    return list;
  }, [data]);

  const totals = useMemo(() => {
    let sessions = 0;
    let covered = 0;
    let recordings = 0;
    for (const e of events) {
      recordings += e.recordings;
      for (const s of e.sessions) {
        sessions++;
        if (s.recordings > 0) covered++;
      }
    }
    return { sessions, covered, recordings };
  }, [events]);

  const layout = useMemo(() => slotLayout(events), [events]);
  const hasSprints = events.some((e) =>
    e.sessions.some((s) => s.session.session_type === ST.Sprint)
  );

  const visible = events
    .map((e, i) => ({ e, slots: layout?.[i] }))
    .filter(({ e }) => !gapsOnly || e.sessions.some((s) => s.recordings === 0));

  function downloadMissing() {
    const blob = new Blob([missingCSV(events, intl)], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${series.name} ${season} missing sessions.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }

  if (error) return <ErrorMessage error={error.message} />;
  if (loading && !data) return <LoadingIndicator />;

  const synced = series.data_source === "jolpica-f1";
  const resultsHidden = shield && !showResults;

  const header = (
    <Panel className="pw-stats">
      <div className="pw-stat">
        <b>
          {totals.covered}
          <span className="pw-muted"> / {totals.sessions}</span>
        </b>
        <span className="pw-muted">
          <FormattedMessage id="sports.sessions_covered_label" />
        </span>
      </div>
      <div className="pw-stat">
        <b>{totals.recordings}</b>
        <span className="pw-muted">
          <FormattedMessage id="sports.recordings" />
        </span>
      </div>
      <div className="pw-stat">
        <b>{events.length}</b>
        <span className="pw-muted">
          <FormattedMessage id="sports.events" />
        </span>
      </div>
      <div className="pw-stat pw-stat-actions">
        {synced && (
          <span className="pw-muted pw-small">
            <FormattedMessage id="sports.calendar_from_jolpica" />
          </span>
        )}
        <div className="pw-actions">
          {synced && <SyncButton seasons={[season]} onDone={() => refetch()} />}
          <Button
            variant="secondary"
            onClick={downloadMissing}
            disabled={totals.covered === totals.sessions}
          >
            <Icon icon={faDownload} />{" "}
            <FormattedMessage id="sports.export_missing" />
          </Button>
          <Button variant="secondary" onClick={() => setEditing("new")}>
            <Icon icon={faPlus} /> <FormattedMessage id="sports.add_event" />
          </Button>
        </div>
      </div>
    </Panel>
  );

  return (
    <>
      {header}
      {editing && (
        <EventDialog
          seriesID={series.id}
          season={season}
          event={editing === "new" ? undefined : editing}
          onClose={() => setEditing(undefined)}
        />
      )}
      {events.length === 0 ? (
        <EmptyState
          title={<FormattedMessage id="sports.no_events" values={{ season }} />}
        >
          <p className="pw-muted">
            <FormattedMessage
              id={
                synced ? "sports.no_events_synced" : "sports.no_events_manual"
              }
            />
          </p>
        </EmptyState>
      ) : (
        <>
          <div className="pw-toolbar">
            <div className="pw-chips">
              <button
                type="button"
                className={cx("pw-chip", { on: gapsOnly })}
                aria-pressed={gapsOnly}
                onClick={() => setGapsOnly(!gapsOnly)}
              >
                <FormattedMessage id="sports.only_gaps" />
              </button>
              {shield && (
                <button
                  type="button"
                  className={cx("pw-chip", { on: showResults })}
                  aria-pressed={showResults}
                  onClick={() => setShowResults(!showResults)}
                >
                  <Icon icon={faEye} />{" "}
                  <FormattedMessage id="sports.show_results" />
                </button>
              )}
            </div>
            <div className="pw-legend">
              <span>
                <span className="pw-swatch pw-cell-multi">3</span>
                <FormattedMessage id="sports.legend_multi" />
              </span>
              <span>
                <span className="pw-swatch pw-cell-have">1</span>
                <FormattedMessage id="sports.legend_one" />
              </span>
              <span>
                <span className="pw-swatch pw-cell-part">½</span>
                <FormattedMessage id="sports.legend_partial" />
              </span>
              <span>
                <span className="pw-swatch pw-cell-miss">+</span>
                <FormattedMessage id="sports.legend_missing" />
              </span>
            </div>
          </div>

          <Panel className="pw-scroll-x">
            <table className={cx("pw-season", { "pw-season-free": !layout })}>
              <thead>
                <tr>
                  <th scope="col">
                    <FormattedMessage id="sports.round_short" />
                  </th>
                  <th scope="col">
                    <FormattedMessage id="sports.event" />
                  </th>
                  <th scope="col">
                    <FormattedMessage id="sports.date" />
                  </th>
                  {layout ? (
                    <>
                      <th scope="col">
                        {intl.formatMessage({
                          id: sessionMessageID(ST.Practice_1),
                        })}
                      </th>
                      <th scope="col">
                        {hasSprints
                          ? "FP2 · SQ"
                          : intl.formatMessage({
                              id: sessionMessageID(ST.Practice_2),
                            })}
                      </th>
                      <th scope="col">
                        {hasSprints
                          ? `FP3 · ${intl.formatMessage({ id: sessionMessageID(ST.Sprint) })}`
                          : intl.formatMessage({
                              id: sessionMessageID(ST.Practice_3),
                            })}
                      </th>
                      <th scope="col">
                        {intl.formatMessage({
                          id: sessionMessageID(ST.Qualifying),
                        })}
                      </th>
                      <th scope="col">
                        {intl.formatMessage({ id: sessionMessageID(ST.Race) })}
                      </th>
                    </>
                  ) : (
                    <th scope="col">
                      <FormattedMessage id="sports.sessions" />
                    </th>
                  )}
                  <th scope="col">
                    <FormattedMessage id="sports.extras" />
                  </th>
                  <th scope="col">
                    <FormattedMessage id="sports.winner" />
                  </th>
                  <th scope="col">
                    <span className="sr-only">
                      <FormattedMessage id="actions.edit" />
                    </span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {visible.map(({ e, slots }) => {
                  const race = e.sessions.find(
                    (s) => s.session.session_type === ST.Race
                  );
                  const sprint = e.sessions.some(
                    (s) => s.session.session_type === ST.Sprint
                  );
                  return (
                    <tr key={e.event.id}>
                      <td className="pw-mono pw-muted">
                        {e.event.round ?? "–"}
                      </td>
                      <td>
                        <Link
                          to={`/events/${e.event.id}`}
                          className="pw-event-link"
                        >
                          {e.event.name}
                        </Link>
                        {sprint && <span className="pw-sprint">SPRINT</span>}
                      </td>
                      <td className="pw-mono pw-muted pw-small">
                        {shortDate(
                          e.event.end_date ?? e.event.start_date,
                          intl.locale
                        )}
                      </td>
                      {slots ? (
                        slots.map((s, i) => (
                          <td key={i}>
                            <Cell eventID={e.event.id} s={s} />
                          </td>
                        ))
                      ) : (
                        <td>
                          <div className="pw-cell-row">
                            {e.sessions.map((s) => (
                              <Cell
                                key={s.session.id}
                                eventID={e.event.id}
                                s={s}
                              />
                            ))}
                            {e.sessions.length === 0 && (
                              <span className="pw-muted pw-small">
                                <FormattedMessage id="sports.no_sessions" />
                              </span>
                            )}
                          </div>
                        </td>
                      )}
                      <td className="pw-mono pw-muted pw-small">
                        {e.extras ? `+${e.extras}` : "—"}
                      </td>
                      <td className="pw-small">
                        {race?.session.winner ? (
                          resultsHidden ? (
                            <span
                              className="pw-hidden-result"
                              title={intl.formatMessage({
                                id: "sports.result_hidden",
                              })}
                            >
                              •••
                            </span>
                          ) : (
                            race.session.winner
                          )
                        ) : (
                          <span className="pw-muted">—</span>
                        )}
                      </td>
                      <td>
                        <Button
                          variant="link"
                          className="pw-icon-btn"
                          aria-label={intl.formatMessage({
                            id: "sports.edit_event",
                          })}
                          onClick={() => setEditing(e.event)}
                        >
                          <Icon icon={faPen} />
                        </Button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </Panel>
        </>
      )}
    </>
  );
};

const SeasonPicker: React.FC<{
  series: GQL.SeriesDataFragment;
  season: number;
}> = ({ series, season }) => {
  const intl = useIntl();
  const history = useHistory();
  const seasons = series.seasons.includes(season)
    ? series.seasons
    : [season, ...series.seasons].sort((a, b) => b - a);
  const i = seasons.indexOf(season);
  // up to seven seasons around the current one, oldest first
  const around = seasons
    .slice(Math.max(0, i - 3), Math.max(0, i - 3) + 7)
    .sort((a, b) => a - b);

  return (
    <div
      className="pw-season-picker"
      role="group"
      aria-label={intl.formatMessage({ id: "sports.season" })}
    >
      {around.map((y) => (
        <Link
          key={y}
          to={`/series/${series.id}/${y}`}
          className={cx("pw-year", { on: y === season })}
          aria-current={y === season ? "page" : undefined}
        >
          {y}
        </Link>
      ))}
      {seasons.length > around.length && (
        <Form.Control
          as="select"
          className="pw-year-select input-control"
          aria-label={intl.formatMessage({ id: "sports.all_seasons" })}
          value=""
          onChange={(e) => {
            const v = e.currentTarget.value;
            if (v) history.push(`/series/${series.id}/${v}`);
          }}
        >
          <option value="">
            {intl.formatMessage({ id: "sports.all_seasons" })}…
          </option>
          {seasons.map((y) => (
            <option key={y} value={y}>
              {y}
            </option>
          ))}
        </Form.Control>
      )}
    </div>
  );
};

export const SeasonPage: React.FC = () => {
  const { id, season: seasonParam } = useParams<{
    id: string;
    season?: string;
  }>();
  const { data, loading, error } = GQL.useFindSeriesQuery({
    variables: { id },
  });
  const [editingSeries, setEditingSeries] = useState(false);
  const series = data?.findSeries;
  const titleProps = useTitleProps(series?.name ?? "", seasonParam ?? "");
  const intl = useIntl();

  if (error) return <ErrorMessage error={error.message} />;
  if (loading && !data) return <LoadingIndicator />;
  if (!series)
    return (
      <ErrorMessage
        error={intl.formatMessage({ id: "sports.series_not_found" })}
      />
    );

  const season = seasonParam
    ? Number.parseInt(seasonParam, 10)
    : defaultSeason(series.seasons);
  if (!seasonParam && season) {
    return <Redirect to={`/series/${series.id}/${season}`} />;
  }

  return (
    <SportsLayout activeSeriesID={series.id}>
      <Helmet {...titleProps} />
      <header className="pw-page-head pw-page-head-end">
        <div className="pw-grow">
          <div className="pw-eyebrow">
            {series.name} · <FormattedMessage id="sports.season" />
            <Button
              variant="link"
              className="pw-icon-btn"
              aria-label={intl.formatMessage({ id: "sports.edit_series" })}
              onClick={() => setEditingSeries(true)}
            >
              <Icon icon={faPen} />
            </Button>
          </div>
          <h1 className="pw-h1">
            {season ? (
              <FormattedMessage
                id="sports.season_title"
                values={{ season, series: series.name }}
              />
            ) : (
              series.name
            )}
          </h1>
        </div>
        {season ? <SeasonPicker series={series} season={season} /> : null}
      </header>
      {editingSeries && (
        <SeriesDialog series={series} onClose={() => setEditingSeries(false)} />
      )}
      {season ? (
        <SeasonGrid series={series} season={season} />
      ) : (
        <EmptyState title={<FormattedMessage id="sports.no_seasons" />}>
          {series.data_source === "jolpica-f1" ? (
            <SyncButton variant="primary" />
          ) : (
            <p className="pw-muted">
              <FormattedMessage id="sports.no_seasons_manual" />
            </p>
          )}
          <SeasonFirstEvent series={series} />
        </EmptyState>
      )}
    </SportsLayout>
  );
};

const SeasonFirstEvent: React.FC<{ series: GQL.SeriesDataFragment }> = ({
  series,
}) => {
  const [adding, setAdding] = useState(false);
  return (
    <>
      <Button variant="secondary" onClick={() => setAdding(true)}>
        <Icon icon={faPlus} /> <FormattedMessage id="sports.add_event" />
      </Button>
      {adding && (
        <EventDialog
          seriesID={series.id}
          season={new Date().getFullYear()}
          onClose={() => setAdding(false)}
        />
      )}
    </>
  );
};

// /seasons opens the main series.
export const SeasonsRedirect: React.FC = () => {
  const { data, loading } = GQL.useAllSeriesQuery();
  if (loading && !data) return <LoadingIndicator />;
  const primary = pickPrimarySeries(data?.allSeries ?? []);
  if (!primary) return <Redirect to="/" />;
  return <Redirect to={`/series/${primary.id}`} />;
};
