import React, { useEffect, useMemo, useState } from "react";
import { Button, Form } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { Link, useHistory, useLocation } from "react-router-dom";
import { Helmet } from "react-helmet";
import { gql, useQuery } from "@apollo/client";
import cx from "classnames";
import {
  faCheck,
  faFilter,
  faMagnifyingGlass,
  faSortDown,
  faSortUp,
  faXmark,
} from "@fortawesome/free-solid-svg-icons";
import * as GQL from "src/core/generated-graphql";
import { Icon } from "src/components/Shared/Icon";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { useTitleProps } from "src/hooks/title";
import { SportsLayout } from "./SportsLayout";
import { FeedTag, LanguageTag, Panel, SourceTag, Stars } from "./common";
import {
  allCompleteness,
  allFeedTypes,
  allSources,
  completenessMessageID,
  formatDuration,
  languageName,
  qualityLabel,
  sessionMessageID,
  sourceMessageID,
  feedMessageID,
  shortEventName,
} from "./utils";

const ST = GQL.SessionType;

// session facet options; practice covers every practice session
const sessionGroups: { key: string; types: GQL.SessionType[] }[] = [
  { key: ST.Race, types: [ST.Race] },
  { key: ST.Qualifying, types: [ST.Qualifying] },
  { key: ST.Sprint, types: [ST.Sprint] },
  { key: ST.SprintQualifying, types: [ST.SprintQualifying] },
  {
    key: ST.Practice,
    types: [ST.Practice_1, ST.Practice_2, ST.Practice_3, ST.Practice],
  },
];

const ORIGINAL = "original";
const PER_PAGE = 50;

type Group = "series" | "session" | "lang" | "feed" | "source" | "completeness";
const groups: Group[] = [
  "series",
  "session",
  "lang",
  "feed",
  "source",
  "completeness",
];

interface IState {
  q: string;
  sel: Record<Group, string[]>;
  from: string;
  to: string;
  rating: string;
  sort: string;
  dir: GQL.SortDirectionEnum;
  page: number;
}

function parseState(search: string): IState {
  const p = new URLSearchParams(search);
  const list = (k: string) =>
    p.get(k) ? p.get(k)!.split(",").filter(Boolean) : [];
  const sel = {} as Record<Group, string[]>;
  for (const g of groups) sel[g] = list(g);
  return {
    q: p.get("q") ?? "",
    sel,
    from: p.get("from") ?? "",
    to: p.get("to") ?? "",
    rating: p.get("rating") ?? "",
    sort: p.get("sort") ?? "event",
    dir:
      p.get("dir") === "asc"
        ? GQL.SortDirectionEnum.Asc
        : GQL.SortDirectionEnum.Desc,
    page: Math.max(1, Number.parseInt(p.get("page") ?? "1", 10) || 1),
  };
}

function stateToSearch(s: IState) {
  const p = new URLSearchParams();
  if (s.q) p.set("q", s.q);
  for (const g of groups) if (s.sel[g].length) p.set(g, s.sel[g].join(","));
  if (s.from) p.set("from", s.from);
  if (s.to) p.set("to", s.to);
  if (s.rating) p.set("rating", s.rating);
  if (s.sort !== "event") p.set("sort", s.sort);
  if (s.dir === GQL.SortDirectionEnum.Asc) p.set("dir", "asc");
  if (s.page > 1) p.set("page", String(s.page));
  return p.toString();
}

const escapeRegex = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

// builds the scene filter, leaving out one group when counting its options
function buildFilter(s: IState, skip?: Group): GQL.SceneFilterType {
  const f: GQL.SceneFilterType = {};
  const inc = GQL.CriterionModifier.Includes;
  const sel = (g: Group) => (g === skip ? [] : s.sel[g]);

  if (sel("series").length) f.series = { value: sel("series"), modifier: inc };
  if (sel("session").length) {
    const types = sessionGroups
      .filter((g) => sel("session").includes(g.key))
      .flatMap((g) => g.types);
    f.session_type = { value: types, modifier: inc };
  }
  const langs = sel("lang");
  if (langs.includes(ORIGINAL)) {
    f.language = { value: "", modifier: GQL.CriterionModifier.IsNull };
  } else if (langs.length) {
    f.language = {
      value: `^(${langs.map(escapeRegex).join("|")})$`,
      modifier: GQL.CriterionModifier.MatchesRegex,
    };
  }
  if (sel("feed").length)
    f.feed_type = { value: sel("feed") as GQL.FeedType[], modifier: inc };
  if (sel("source").length)
    f.source = { value: sel("source") as GQL.RecordingSource[], modifier: inc };
  if (sel("completeness").length)
    f.completeness = {
      value: sel("completeness") as GQL.Completeness[],
      modifier: inc,
    };
  const from = Number.parseInt(s.from, 10);
  const to = Number.parseInt(s.to, 10);
  if (from || to) {
    f.season = {
      value: from || 1900,
      value2: to || 2200,
      modifier: GQL.CriterionModifier.Between,
    };
  }
  const rating = Number.parseInt(s.rating, 10);
  if (rating)
    f.rating100 = {
      value: rating - 1,
      modifier: GQL.CriterionModifier.GreaterThan,
    };
  return f;
}

interface IOption {
  group: Group;
  value: string;
  label: string;
}

// One request counting the recordings each facet option would show.
function useFacetCounts(state: IState, options: IOption[]) {
  const { query, variables } = useMemo(() => {
    const defs: string[] = [];
    const fields: string[] = [];
    const vars: Record<string, unknown> = {};
    options.forEach((o, i) => {
      const f = buildFilter(
        { ...state, sel: { ...state.sel, [o.group]: [o.value] } },
        undefined
      );
      // other groups stay applied; this group is replaced by the option
      vars[`f${i}`] = f;
      defs.push(`$f${i}: SceneFilterType`);
      fields.push(
        `f${i}: findScenes(scene_filter: $f${i}, filter: { q: $q, per_page: 1 }) { count }`
      );
    });
    vars.q = state.q || null;
    const doc = options.length
      ? gql(
          `query RecordingFacetCounts($q: String, ${defs.join(", ")}) { ${fields.join(" ")} }`
        )
      : gql("query RecordingFacetCountsEmpty { __typename }");
    return { query: doc, variables: vars };
  }, [state, options]);

  const { data } = useQuery<Record<string, { count: number }>>(query, {
    variables,
    skip: options.length === 0,
  });

  return (i: number) => data?.[`f${i}`]?.count;
}

const SortHeader: React.FC<{
  id: string;
  state: IState;
  onSort: (sort: string) => void;
  children: React.ReactNode;
}> = ({ id, state, onSort, children }) => {
  const active = state.sort === id;
  return (
    <th
      scope="col"
      aria-sort={
        active
          ? state.dir === GQL.SortDirectionEnum.Asc
            ? "ascending"
            : "descending"
          : undefined
      }
    >
      <button type="button" className="pw-sort" onClick={() => onSort(id)}>
        {children}
        {active && (
          <Icon
            icon={
              state.dir === GQL.SortDirectionEnum.Asc ? faSortUp : faSortDown
            }
          />
        )}
      </button>
    </th>
  );
};

export const RecordingsBrowser: React.FC = () => {
  const intl = useIntl();
  const history = useHistory();
  const location = useLocation();
  const titleProps = useTitleProps({ id: "sports.recordings" });
  const state = useMemo(() => parseState(location.search), [location.search]);
  const [search, setSearch] = useState(state.q);
  // on narrow screens the filters fold away behind a button
  const [filtersOpen, setFiltersOpen] = useState(false);
  useEffect(() => setSearch(state.q), [state.q]);

  const { data: seriesData } = GQL.useAllSeriesQuery();
  const { data: langData } = GQL.useRecordingLanguagesQuery();

  function update(change: Partial<IState>) {
    const next = { ...state, page: 1, ...change };
    history.replace({
      pathname: location.pathname,
      search: stateToSearch(next),
    });
  }

  function toggle(group: Group, value: string) {
    let cur = state.sel[group];
    if (cur.includes(value)) {
      cur = cur.filter((v) => v !== value);
    } else if (group === "lang") {
      // original audio cannot be combined with commentary languages
      cur =
        value === ORIGINAL
          ? [ORIGINAL]
          : [...cur.filter((v) => v !== ORIGINAL), value];
    } else {
      cur = [...cur, value];
    }
    update({ sel: { ...state.sel, [group]: cur } });
  }

  const options = useMemo<IOption[]>(() => {
    const ret: IOption[] = [];
    for (const s of seriesData?.allSeries ?? [])
      ret.push({ group: "series", value: s.id, label: s.name });
    for (const g of sessionGroups)
      ret.push({
        group: "session",
        value: g.key,
        label: intl.formatMessage({
          id:
            g.key === ST.Practice
              ? "sports.practice_any"
              : sessionMessageID(g.key as GQL.SessionType),
        }),
      });
    for (const l of langData?.recordingLanguages ?? [])
      ret.push({
        group: "lang",
        value: l,
        label: languageName(l, intl.locale),
      });
    ret.push({
      group: "lang",
      value: ORIGINAL,
      label: intl.formatMessage({ id: "sports.original_audio" }),
    });
    for (const f of allFeedTypes)
      ret.push({
        group: "feed",
        value: f,
        label: intl.formatMessage({ id: feedMessageID(f) }),
      });
    for (const s of allSources)
      ret.push({
        group: "source",
        value: s,
        label: intl.formatMessage({ id: sourceMessageID(s) }),
      });
    for (const c of allCompleteness)
      ret.push({
        group: "completeness",
        value: c,
        label: intl.formatMessage({ id: completenessMessageID(c) }),
      });
    return ret;
  }, [seriesData, langData, intl]);

  const countOf = useFacetCounts(state, options);

  const { data, loading, previousData } = GQL.useFindRecordingsQuery({
    variables: {
      filter: {
        q: state.q || undefined,
        per_page: PER_PAGE,
        page: state.page,
        sort: state.sort,
        direction: state.dir,
      },
      scene_filter: buildFilter(state),
    },
  });
  const result = (data ?? previousData)?.findScenes;
  const rows = result?.scenes ?? [];
  const count = result?.count ?? 0;
  const pages = Math.max(1, Math.ceil(count / PER_PAGE));

  function onSort(sort: string) {
    if (state.sort === sort) {
      update({
        dir:
          state.dir === GQL.SortDirectionEnum.Asc
            ? GQL.SortDirectionEnum.Desc
            : GQL.SortDirectionEnum.Asc,
      });
    } else {
      update({
        sort,
        dir:
          sort === "title"
            ? GQL.SortDirectionEnum.Asc
            : GQL.SortDirectionEnum.Desc,
      });
    }
  }

  const active: { group: Group; value: string; label: string }[] = [];
  for (const g of groups) {
    for (const v of state.sel[g]) {
      const o = options.find((x) => x.group === g && x.value === v);
      active.push({ group: g, value: v, label: o?.label ?? v });
    }
  }

  const filterCount =
    active.length + (state.from || state.to ? 1 : 0) + (state.rating ? 1 : 0);

  return (
    <SportsLayout>
      <Helmet {...titleProps} />
      <header className="pw-page-head">
        <h1 className="pw-h1">
          <FormattedMessage id="sports.recordings" />
        </h1>
        <Form
          className="pw-search"
          role="search"
          onSubmit={(e) => {
            e.preventDefault();
            update({ q: search.trim() });
          }}
        >
          <Icon icon={faMagnifyingGlass} />
          <Form.Control
            type="search"
            value={search}
            aria-label={intl.formatMessage({ id: "sports.search_recordings" })}
            placeholder={intl.formatMessage({
              id: "sports.search_recordings_placeholder",
            })}
            onChange={(e) => setSearch(e.currentTarget.value)}
          />
        </Form>
        <Button
          variant="secondary"
          className="pw-facets-toggle"
          aria-expanded={filtersOpen}
          aria-controls="recording-filters"
          onClick={() => setFiltersOpen(!filtersOpen)}
        >
          <Icon icon={faFilter} /> <FormattedMessage id="sports.filters" />
          {filterCount > 0 && (
            <span className="pw-nav-badge">{filterCount}</span>
          )}
        </Button>
      </header>

      <div className="pw-browse">
        <aside
          id="recording-filters"
          className={cx("pw-facets pw-panel", { open: filtersOpen })}
          aria-label={intl.formatMessage({ id: "sports.filters" })}
        >
          {groups.map((g) => {
            const opts = options
              .map((o, i) => ({ ...o, i }))
              .filter((o) => o.group === g)
              .filter(
                (o) =>
                  state.sel[g].includes(o.value) ||
                  countOf(o.i) !== 0 ||
                  g === "series" ||
                  g === "session"
              );
            if (opts.length === 0) return null;
            return (
              <div className="pw-facet" key={g}>
                <h2 className="pw-facet-head">
                  <FormattedMessage id={`sports.facet.${g}`} />
                </h2>
                {opts.map((o) => {
                  const on = state.sel[g].includes(o.value);
                  const n = countOf(o.i);
                  return (
                    <button
                      type="button"
                      key={o.value}
                      className={cx("pw-opt", { on })}
                      aria-pressed={on}
                      onClick={() => toggle(g, o.value)}
                    >
                      <span className="pw-box" aria-hidden="true">
                        {on && <Icon icon={faCheck} />}
                      </span>
                      <span className="pw-grow pw-ellipsis">{o.label}</span>
                      <span className="pw-mono pw-muted pw-small">
                        {n === undefined ? "" : intl.formatNumber(n)}
                      </span>
                    </button>
                  );
                })}
              </div>
            );
          })}
          <div className="pw-facet">
            <h2 className="pw-facet-head">
              <FormattedMessage id="sports.facet.season" />
            </h2>
            <div className="pw-range">
              <Form.Control
                className="text-input pw-mono"
                type="number"
                min={1900}
                max={2200}
                aria-label={intl.formatMessage({ id: "sports.season_from" })}
                placeholder="1950"
                defaultValue={state.from}
                onBlur={(e) => update({ from: e.currentTarget.value })}
              />
              <span className="pw-muted">–</span>
              <Form.Control
                className="text-input pw-mono"
                type="number"
                min={1900}
                max={2200}
                aria-label={intl.formatMessage({ id: "sports.season_to" })}
                placeholder={String(new Date().getFullYear())}
                defaultValue={state.to}
                onBlur={(e) => update({ to: e.currentTarget.value })}
              />
            </div>
          </div>
          <div className="pw-facet">
            <h2 className="pw-facet-head">
              <label htmlFor="rating-min">
                <FormattedMessage id="sports.facet.rating" />
              </label>
            </h2>
            <Form.Control
              id="rating-min"
              as="select"
              className="input-control"
              value={state.rating}
              onChange={(e) => update({ rating: e.currentTarget.value })}
            >
              <option value="">
                {intl.formatMessage({ id: "sports.any_rating" })}
              </option>
              {[5, 4, 3, 2].map((n) => (
                <option key={n} value={n * 20}>
                  {intl.formatMessage(
                    { id: "sports.rating_at_least" },
                    { stars: n }
                  )}
                </option>
              ))}
            </Form.Control>
          </div>
        </aside>

        <section className="pw-results">
          <div className="pw-toolbar">
            <span className="pw-mono">
              <b>{intl.formatNumber(count)}</b>{" "}
              <span className="pw-muted">
                <FormattedMessage id="sports.recordings_lower" />
              </span>
              {result?.duration ? (
                <span className="pw-muted">
                  {" "}
                  · {formatDuration(result.duration)}
                </span>
              ) : null}
            </span>
            {state.q && (
              <button
                type="button"
                className="pw-chip on"
                onClick={() => update({ q: "" })}
              >
                “{state.q}” <Icon icon={faXmark} />
              </button>
            )}
            {active.map((a) => (
              <button
                type="button"
                key={`${a.group}-${a.value}`}
                className="pw-chip on"
                onClick={() => toggle(a.group, a.value)}
                aria-label={intl.formatMessage(
                  { id: "sports.remove_filter" },
                  { filter: a.label }
                )}
              >
                {a.label} <Icon icon={faXmark} />
              </button>
            ))}
            {(active.length > 0 ||
              state.q ||
              state.from ||
              state.to ||
              state.rating) && (
              <Button
                variant="link"
                onClick={() => history.replace({ pathname: location.pathname })}
              >
                <FormattedMessage id="sports.clear_filters" />
              </Button>
            )}
          </div>

          <Panel className="pw-scroll-x">
            <table className="pw-table">
              <thead>
                <tr>
                  <SortHeader id="event" state={state} onSort={onSort}>
                    <FormattedMessage id="sports.event" />
                  </SortHeader>
                  <th scope="col">
                    <FormattedMessage id="sports.session" />
                  </th>
                  <th scope="col">
                    <FormattedMessage id="sports.broadcaster" />
                  </th>
                  <th scope="col">
                    <FormattedMessage id="sports.lang" />
                  </th>
                  <th scope="col">
                    <FormattedMessage id="sports.feed" />
                  </th>
                  <SortHeader id="duration" state={state} onSort={onSort}>
                    <FormattedMessage id="sports.length" />
                  </SortHeader>
                  <SortHeader id="rating" state={state} onSort={onSort}>
                    <FormattedMessage id="sports.rating" />
                  </SortHeader>
                  <SortHeader id="created_at" state={state} onSort={onSort}>
                    <FormattedMessage id="sports.added" />
                  </SortHeader>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr
                    key={r.id}
                    className={cx({
                      "pw-partial": r.completeness === GQL.Completeness.Partial,
                    })}
                  >
                    <td>
                      <Link
                        to={`/scenes/${r.id}`}
                        className={cx("pw-strong", { "pw-file": !r.event })}
                      >
                        {r.event
                          ? `${r.event.season} ${shortEventName(r.event.name)}`
                          : r.title || r.files[0]?.basename}
                      </Link>
                      {r.event && (
                        <div className="pw-muted pw-small">
                          {r.event.series.short_name || r.event.series.name}
                          {r.event.round ? ` · R${r.event.round}` : ""}
                        </div>
                      )}
                    </td>
                    <td className="pw-muted">
                      {r.session
                        ? intl.formatMessage({
                            id: sessionMessageID(r.session.session_type),
                          })
                        : r.event
                          ? intl.formatMessage({ id: "sports.extra" })
                          : "—"}
                    </td>
                    <td className="pw-nowrap">{r.studio?.name ?? "—"}</td>
                    <td>
                      <LanguageTag language={r.language} />
                    </td>
                    <td>
                      <div className="pw-tag-list">
                        <FeedTag feed={r.feed_type} subject={r.feed_subject} />
                        <SourceTag source={r.source} />
                      </div>
                    </td>
                    <td className="pw-mono pw-muted pw-small pw-nowrap">
                      {formatDuration(r.files[0]?.duration)}
                      <div>{qualityLabel(r.files[0])}</div>
                    </td>
                    <td>
                      <Stars rating100={r.rating100} />
                    </td>
                    <td className="pw-mono pw-muted pw-small pw-nowrap">
                      {r.created_at.slice(0, 10)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {loading && rows.length === 0 && <LoadingIndicator inline small />}
            {!loading && rows.length === 0 && (
              <p className="pw-panel-sub pw-muted">
                <FormattedMessage id="sports.no_matches" />
              </p>
            )}
          </Panel>

          {pages > 1 && (
            <nav
              className="pw-pager"
              aria-label={intl.formatMessage({ id: "sports.pages" })}
            >
              <Button
                variant="secondary"
                disabled={state.page <= 1}
                onClick={() => update({ page: state.page - 1 })}
              >
                <FormattedMessage id="sports.previous" />
              </Button>
              <span className="pw-mono pw-muted">
                {state.page} / {pages}
              </span>
              <Button
                variant="secondary"
                disabled={state.page >= pages}
                onClick={() => update({ page: state.page + 1 })}
              >
                <FormattedMessage id="sports.next" />
              </Button>
            </nav>
          )}
        </section>
      </div>
    </SportsLayout>
  );
};

export default RecordingsBrowser;
