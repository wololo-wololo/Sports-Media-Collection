import React, { useEffect, useMemo, useState } from "react";
import { Button, Col, Form, Row } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { faCrosshairs, faFloppyDisk } from "@fortawesome/free-solid-svg-icons";
import * as GQL from "src/core/generated-graphql";
import { Icon } from "src/components/Shared/Icon";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { useToast } from "src/hooks/Toast";
import {
  allCompleteness,
  allFeedTypes,
  allSources,
  completenessMessageID,
  eventLabel,
  feedMessageID,
  formatDuration,
  languageName,
  sessionMessageID,
  sessionSortKey,
  sourceMessageID,
} from "./utils";

const commonLanguages = [
  "en",
  "de",
  "nl",
  "fr",
  "it",
  "es",
  "pt",
  "ja",
  "pl",
  "fi",
  "hu",
  "sv",
  "da",
];

interface IRecordingPanel {
  sceneID: string;
  getPlayerPosition?: () => number | undefined;
}

interface IValues {
  seriesID: string;
  season: string;
  eventID: string;
  sessionID: string;
  language: string;
  feed: string;
  subject: string;
  source: string;
  completeness: string;
  commentators: string;
  sync: string;
}

function initial(r?: GQL.RecordingDataFragment): IValues {
  return {
    seriesID: r?.event?.series.id ?? "",
    season: r?.event ? String(r.event.season) : "",
    eventID: r?.event?.id ?? "",
    sessionID: r?.session?.id ?? "",
    language: r?.language ?? "",
    feed: r?.feed_type ?? "",
    subject: r?.feed_subject ?? "",
    source: r?.source ?? "",
    completeness: r?.completeness ?? "",
    commentators: r?.commentators ?? "",
    sync: r?.sync_offset != null ? String(r.sync_offset) : "",
  };
}

export const RecordingPanel: React.FC<IRecordingPanel> = ({
  sceneID,
  getPlayerPosition,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const { data, loading } = GQL.useFindRecordingQuery({
    variables: { id: sceneID },
  });
  const recording = data?.findScene ?? undefined;
  const [v, setV] = useState<IValues>(initial());
  useEffect(() => setV(initial(recording)), [recording]);

  const { data: seriesData } = GQL.useAllSeriesQuery();
  const series = seriesData?.allSeries ?? [];
  const selectedSeries = series.find((s) => s.id === v.seriesID);
  const { data: eventsData } = GQL.useFindEventsQuery({
    variables: { series_id: v.seriesID, season: Number.parseInt(v.season, 10) },
    skip: !v.seriesID || !v.season,
  });
  const { data: sessionsData } = GQL.useFindEventSessionsQuery({
    variables: { id: v.eventID },
    skip: !v.eventID,
  });
  const { data: langData } = GQL.useRecordingLanguagesQuery();
  const languages = useMemo(
    () => [
      ...new Set([...(langData?.recordingLanguages ?? []), ...commonLanguages]),
    ],
    [langData]
  );

  const [save, { loading: saving }] = GQL.useRecordingUpdateMutation({
    refetchQueries: [
      "FindRecording",
      "FindScene",
      "SeasonCoverage",
      "SeriesCoverage",
      "AllSeries",
      "SportsImportCount",
      "SportsImportSuggestions",
    ],
  });

  if (loading && !recording) return <LoadingIndicator />;
  if (!recording) return null;

  const set =
    (k: keyof IValues) =>
    (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
      const value = e.currentTarget.value;
      setV((cur) => {
        const next = { ...cur, [k]: value };
        // choosing a broader level clears the narrower ones
        if (k === "seriesID")
          Object.assign(next, { season: "", eventID: "", sessionID: "" });
        if (k === "season") Object.assign(next, { eventID: "", sessionID: "" });
        if (k === "eventID") next.sessionID = "";
        return next;
      });
    };

  async function onSave() {
    const sync = v.sync.trim() === "" ? null : Number.parseFloat(v.sync);
    try {
      await save({
        variables: {
          input: {
            id: sceneID,
            event_id: v.eventID || null,
            session_id: v.sessionID || null,
            language: v.language.trim() || null,
            feed_type: (v.feed as GQL.FeedType) || null,
            feed_subject: v.subject.trim() || null,
            source: (v.source as GQL.RecordingSource) || null,
            completeness: (v.completeness as GQL.Completeness) || null,
            commentators: v.commentators.trim() || null,
            sync_offset: sync !== null && !Number.isNaN(sync) ? sync : null,
          },
        },
      });
      Toast.success(intl.formatMessage({ id: "sports.recording_saved" }));
    } catch (e) {
      Toast.error(e);
    }
  }

  const sessions = [...(sessionsData?.findEvent?.sessions ?? [])].sort(
    (a, b) => sessionSortKey(a) - sessionSortKey(b)
  );
  const seasons = selectedSeries?.seasons ?? [];
  const changed = JSON.stringify(v) !== JSON.stringify(initial(recording));

  return (
    <Form
      className="pw-recording-form"
      onSubmit={(e) => {
        e.preventDefault();
        onSave();
      }}
    >
      {recording.event && (
        <p className="pw-recording-summary">
          <Link to={`/events/${recording.event.id}`}>
            {eventLabel(recording.event)}
          </Link>
          {recording.session && (
            <>
              {" "}
              ·{" "}
              {intl.formatMessage({
                id: sessionMessageID(recording.session.session_type),
              })}
            </>
          )}
          {recording.studio && <> · {recording.studio.name}</>}
        </p>
      )}

      <fieldset>
        <legend className="pw-legend-title">
          <FormattedMessage id="sports.event" />
        </legend>
        <Row>
          <Col sm={6}>
            <Form.Group controlId="rec-series">
              <Form.Label>
                <FormattedMessage id="sports.facet.series" />
              </Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={v.seriesID}
                onChange={set("seriesID")}
              >
                <option value="">—</option>
                {series.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </Form.Control>
            </Form.Group>
          </Col>
          <Col sm={6}>
            <Form.Group controlId="rec-season">
              <Form.Label>
                <FormattedMessage id="sports.season" />
              </Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={v.season}
                onChange={set("season")}
                disabled={!v.seriesID}
              >
                <option value="">—</option>
                {seasons.map((y) => (
                  <option key={y} value={y}>
                    {y}
                  </option>
                ))}
              </Form.Control>
            </Form.Group>
          </Col>
        </Row>
        <Row>
          <Col sm={6}>
            <Form.Group controlId="rec-event">
              <Form.Label>
                <FormattedMessage id="sports.event" />
              </Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={v.eventID}
                onChange={set("eventID")}
                disabled={!v.season}
              >
                <option value="">—</option>
                {(eventsData?.findEvents ?? []).map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.round ? `R${e.round} · ` : ""}
                    {e.name}
                  </option>
                ))}
              </Form.Control>
            </Form.Group>
          </Col>
          <Col sm={6}>
            <Form.Group controlId="rec-session">
              <Form.Label>
                <FormattedMessage id="sports.session" />
              </Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={v.sessionID}
                onChange={set("sessionID")}
                disabled={!v.eventID}
              >
                <option value="">
                  {intl.formatMessage({ id: "sports.no_session_extra" })}
                </option>
                {sessions.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name ||
                      intl.formatMessage({
                        id: sessionMessageID(s.session_type),
                      })}
                  </option>
                ))}
              </Form.Control>
            </Form.Group>
          </Col>
        </Row>
      </fieldset>

      <fieldset>
        <legend className="pw-legend-title">
          <FormattedMessage id="sports.coverage" />
        </legend>
        <Row>
          <Col sm={6}>
            <Form.Group controlId="rec-language">
              <Form.Label>
                <FormattedMessage id="sports.commentary_language" />
              </Form.Label>
              <Form.Control
                className="text-input"
                list="rec-languages"
                value={v.language}
                onChange={set("language")}
                placeholder={intl.formatMessage({
                  id: "sports.language_placeholder",
                })}
              />
              <datalist id="rec-languages">
                {languages.map((l) => (
                  <option key={l} value={l}>
                    {languageName(l, intl.locale)}
                  </option>
                ))}
              </datalist>
              <Form.Text className="pw-muted">
                {v.language
                  ? languageName(v.language, intl.locale)
                  : intl.formatMessage({ id: "sports.original_audio" })}
              </Form.Text>
            </Form.Group>
          </Col>
          <Col sm={6}>
            <Form.Group controlId="rec-commentators">
              <Form.Label>
                <FormattedMessage id="sports.commentators" />
              </Form.Label>
              <Form.Control
                className="text-input"
                value={v.commentators}
                onChange={set("commentators")}
              />
            </Form.Group>
          </Col>
        </Row>
        <Row>
          <Col sm={6}>
            <Form.Group controlId="rec-feed">
              <Form.Label>
                <FormattedMessage id="sports.feed" />
              </Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={v.feed}
                onChange={set("feed")}
              >
                <option value="">—</option>
                {allFeedTypes.map((f) => (
                  <option key={f} value={f}>
                    {intl.formatMessage({ id: feedMessageID(f) })}
                  </option>
                ))}
              </Form.Control>
            </Form.Group>
          </Col>
          <Col sm={6}>
            <Form.Group controlId="rec-subject">
              <Form.Label>
                <FormattedMessage id="sports.feed_subject" />
              </Form.Label>
              <Form.Control
                className="text-input"
                value={v.subject}
                onChange={set("subject")}
                placeholder={intl.formatMessage({
                  id: "sports.feed_subject_placeholder",
                })}
              />
            </Form.Group>
          </Col>
        </Row>
        <Row>
          <Col sm={6}>
            <Form.Group controlId="rec-source">
              <Form.Label>
                <FormattedMessage id="sports.source" />
              </Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={v.source}
                onChange={set("source")}
              >
                <option value="">—</option>
                {allSources.map((s) => (
                  <option key={s} value={s}>
                    {intl.formatMessage({ id: sourceMessageID(s) })}
                  </option>
                ))}
              </Form.Control>
            </Form.Group>
          </Col>
          <Col sm={6}>
            <Form.Group controlId="rec-completeness">
              <Form.Label>
                <FormattedMessage id="sports.completeness_label" />
              </Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={v.completeness}
                onChange={set("completeness")}
              >
                <option value="">—</option>
                {allCompleteness.map((c) => (
                  <option key={c} value={c}>
                    {intl.formatMessage({ id: completenessMessageID(c) })}
                  </option>
                ))}
              </Form.Control>
            </Form.Group>
          </Col>
        </Row>
      </fieldset>

      <fieldset>
        <legend className="pw-legend-title">
          <FormattedMessage id="sports.sync_point" />
        </legend>
        <Form.Group controlId="rec-sync">
          <Form.Label>
            <FormattedMessage id="sports.sync_offset" />
          </Form.Label>
          <div className="pw-inline">
            <Form.Control
              className="text-input pw-mono pw-narrow"
              type="number"
              step="0.1"
              min={0}
              value={v.sync}
              onChange={set("sync")}
            />
            {getPlayerPosition && (
              <Button
                variant="secondary"
                onClick={() => {
                  const pos = getPlayerPosition();
                  if (pos !== undefined) setV({ ...v, sync: pos.toFixed(1) });
                }}
              >
                <Icon icon={faCrosshairs} />{" "}
                <FormattedMessage id="sports.use_player_position" />
              </Button>
            )}
            {v.sync && (
              <span className="pw-mono pw-muted">
                {formatDuration(Number.parseFloat(v.sync))}
              </span>
            )}
          </div>
          <Form.Text className="pw-muted">
            <FormattedMessage id="sports.sync_offset_help" />
          </Form.Text>
        </Form.Group>
      </fieldset>

      <Button type="submit" variant="primary" disabled={saving || !changed}>
        <Icon icon={faFloppyDisk} /> <FormattedMessage id="actions.save" />
      </Button>
    </Form>
  );
};

export default RecordingPanel;
