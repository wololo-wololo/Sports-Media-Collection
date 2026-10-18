import * as GQL from "src/core/generated-graphql";

// Weekend order of the session types, matching the backend.
export const sessionTypeOrder: GQL.SessionType[] = [
  GQL.SessionType.Practice_1,
  GQL.SessionType.Practice_2,
  GQL.SessionType.Practice_3,
  GQL.SessionType.Practice,
  GQL.SessionType.Qualifying,
  GQL.SessionType.SprintQualifying,
  GQL.SessionType.Sprint,
  GQL.SessionType.WarmUp,
  GQL.SessionType.Race,
  GQL.SessionType.Match,
  GQL.SessionType.Other,
];

const sessionCodes: Record<GQL.SessionType, string> = {
  [GQL.SessionType.Practice_1]: "FP1",
  [GQL.SessionType.Practice_2]: "FP2",
  [GQL.SessionType.Practice_3]: "FP3",
  [GQL.SessionType.Practice]: "P",
  [GQL.SessionType.Qualifying]: "Q",
  [GQL.SessionType.SprintQualifying]: "SQ",
  [GQL.SessionType.Sprint]: "S",
  [GQL.SessionType.WarmUp]: "WU",
  [GQL.SessionType.Race]: "R",
  [GQL.SessionType.Match]: "M",
  [GQL.SessionType.Other]: "OT",
};

export function sessionCode(t: GQL.SessionType) {
  return sessionCodes[t] ?? t;
}

// message ids under sports.session_types / feed_types / sources
export function sessionMessageID(t: GQL.SessionType) {
  return `sports.session_types.${t}`;
}

export function feedMessageID(t: GQL.FeedType) {
  return `sports.feed_types.${t}`;
}

export function sourceMessageID(t: GQL.RecordingSource) {
  return `sports.sources.${t}`;
}

export function completenessMessageID(t: GQL.Completeness) {
  return `sports.completeness.${t}`;
}

// display order, as in the schema (the generated enums are alphabetical)
export const allFeedTypes: GQL.FeedType[] = [
  GQL.FeedType.WorldFeed,
  GQL.FeedType.Onboard,
  GQL.FeedType.PitLane,
  GQL.FeedType.DataChannel,
  GQL.FeedType.DriverTracker,
  GQL.FeedType.Studio,
  GQL.FeedType.Highlights,
  GQL.FeedType.Documentary,
  GQL.FeedType.Other,
];
export const allSources: GQL.RecordingSource[] = [
  GQL.RecordingSource.Tv,
  GQL.RecordingSource.Streaming,
  GQL.RecordingSource.Dvd,
  GQL.RecordingSource.BluRay,
  GQL.RecordingSource.Vhs,
  GQL.RecordingSource.Web,
  GQL.RecordingSource.Other,
];
export const allCompleteness: GQL.Completeness[] = [
  GQL.Completeness.Complete,
  GQL.Completeness.Partial,
];

export function sessionSortKey(s: { session_type: GQL.SessionType }) {
  const i = sessionTypeOrder.indexOf(s.session_type);
  return i < 0 ? sessionTypeOrder.length : i;
}

// Display name of a language code in the UI language, e.g. "en" -> "English".
export function languageName(code: string | null | undefined, locale: string) {
  if (!code) return "";
  try {
    const names = new Intl.DisplayNames([locale], { type: "language" });
    return names.of(code) ?? code;
  } catch {
    return code;
  }
}

export function languageTag(code: string | null | undefined) {
  return code ? code.toUpperCase() : "—";
}

export function formatDuration(seconds: number | null | undefined) {
  if (!seconds || seconds < 0) return "";
  const s = Math.round(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const mm = h > 0 ? String(m).padStart(2, "0") : String(m);
  return `${h > 0 ? `${h}:` : ""}${mm}:${String(sec).padStart(2, "0")}`;
}

export function qualityLabel(
  file?: { height?: number | null; frame_rate?: number | null } | null
) {
  if (!file?.height) return "";
  const fps = file.frame_rate ? Math.round(file.frame_rate) : 0;
  return `${file.height}p${fps >= 48 ? fps : ""}`;
}

export function eventLabel(e: {
  season: number;
  name: string;
  round?: number | null;
}) {
  return `${e.season} ${e.name}`;
}

// short form of an event name for tight places: "São Paulo GP"
export function shortEventName(name: string) {
  return name.replace(/\bGrand Prix\b/, "GP");
}

export function seriesColor(s?: { color?: string | null } | null) {
  return s?.color || "#8c96a3";
}

export function dateRange(
  start: string | null | undefined,
  end: string | null | undefined,
  locale: string
) {
  const parse = (d: string) => new Date(`${d}T12:00:00Z`);
  const fmt = (d: string, opts: Intl.DateTimeFormatOptions) =>
    new Intl.DateTimeFormat(locale, { timeZone: "UTC", ...opts }).format(
      parse(d)
    );
  if (!start && !end) return "";
  if (!start || !end || start === end) {
    return fmt((start || end)!, {
      day: "numeric",
      month: "short",
      year: "numeric",
    });
  }
  const sameMonth = start.slice(0, 7) === end.slice(0, 7);
  return `${fmt(start, sameMonth ? { day: "numeric" } : { day: "numeric", month: "short" })}–${fmt(end, { day: "numeric", month: "short", year: "numeric" })}`;
}

export function shortDate(d: string | null | undefined, locale: string) {
  if (!d) return "";
  return new Intl.DateTimeFormat(locale, {
    timeZone: "UTC",
    day: "2-digit",
    month: "short",
  }).format(new Date(`${d}T12:00:00Z`));
}

export function sessionTime(t: string | null | undefined, locale: string) {
  if (!t) return "";
  return new Intl.DateTimeFormat(locale, {
    weekday: "long",
    day: "numeric",
    month: "long",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(t));
}

// The season to open by default: the latest season that has started.
export function defaultSeason(seasons: number[]) {
  const year = new Date().getFullYear();
  const started = seasons.filter((s) => s <= year);
  return started.length > 0 ? Math.max(...started) : seasons[0];
}

export function pickPrimarySeries<
  T extends { name: string; event_count: number },
>(series: T[]) {
  return (
    series.find((s) => s.name === "Formula 1" && s.event_count > 0) ??
    series.find((s) => s.event_count > 0) ??
    series[0]
  );
}

// Recordings sorted so that complete world-feed broadcasts come first.
export function recordingSortKey(r: GQL.RecordingDataFragment) {
  const feed = r.feed_type ? allFeedTypes.indexOf(r.feed_type) : 99;
  const partial = r.completeness === GQL.Completeness.Partial ? 1 : 0;
  return feed * 10 + partial;
}
