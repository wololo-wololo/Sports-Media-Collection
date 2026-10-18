import React, { useCallback, useState } from "react";
import { Button, Form, ProgressBar } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { Helmet } from "react-helmet";
import cx from "classnames";
import {
  faCheck,
  faCompactDisc,
  faEyeSlash,
  faFolderOpen,
  faPen,
  faRotateLeft,
} from "@fortawesome/free-solid-svg-icons";
import * as GQL from "src/core/generated-graphql";
import { mutateMetadataScan } from "src/core/StashService";
import { Icon } from "src/components/Shared/Icon";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { useTitleProps } from "src/hooks/title";
import { useToast } from "src/hooks/Toast";
import { JobFragment, useMonitorJob } from "src/utils/job";
import { SportsLayout } from "./SportsLayout";
import { EmptyState, Panel } from "./common";
import { eventLabel, formatDuration } from "./utils";

// paths come from the server's OS, so split on either separator
function splitAt(path: string) {
  return Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\"));
}

function fileName(path: string) {
  return path.slice(splitAt(path) + 1);
}

function folderName(path: string) {
  return path.slice(0, Math.max(splitAt(path), 0));
}

type Suggestion =
  GQL.SportsImportSuggestionsQuery["sportsImportSuggestions"]["suggestions"][number];

const PER_PAGE = 25;
const refetch = [
  "SportsImportSuggestions",
  "SportsImportCount",
  "SeasonCoverage",
  "SeriesCoverage",
  "AllSeries",
  "FindRecordings",
];

const confidenceClass: Record<GQL.ImportConfidence, string> = {
  [GQL.ImportConfidence.High]: "pw-conf-hi",
  [GQL.ImportConfidence.Medium]: "pw-conf-md",
  [GQL.ImportConfidence.Low]: "pw-conf-lo",
  [GQL.ImportConfidence.None]: "pw-conf-none",
};

const SuggestionCard: React.FC<{
  s: Suggestion;
  dismissed: boolean;
  onChanged: () => void;
}> = ({ s, dismissed, onChanged }) => {
  const intl = useIntl();
  const Toast = useToast();
  const [accept, { loading: accepting }] = GQL.useSportsImportAcceptMutation({
    refetchQueries: refetch,
  });
  const [dismiss, { loading: dismissing }] = GQL.useSportsImportDismissMutation(
    { refetchQueries: refetch }
  );
  const [eventID, setEventID] = useState(s.event?.id ?? "");

  const isDVD = s.source === GQL.RecordingSource.Dvd;
  const file = s.scene.files[0];

  async function onAccept() {
    try {
      await accept({
        variables: {
          input: {
            scene_id: s.scene.id,
            event_id: eventID && eventID !== s.event?.id ? eventID : null,
          },
        },
      });
      onChanged();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onDismiss() {
    try {
      await dismiss({
        variables: {
          input: { scene_ids: [s.scene.id], dismissed: !dismissed },
        },
      });
      onChanged();
    } catch (e) {
      Toast.error(e);
    }
  }

  const choices = s.alternatives.length > 0 ? s.alternatives : [];
  const confLabel = intl.formatMessage({
    id: `sports.confidence.${s.confidence}`,
  });

  return (
    <article className="pw-import-item">
      <div className="pw-import-head">
        <span className={cx("pw-conf", confidenceClass[s.confidence])}>
          {confLabel}
        </span>
        <span className="pw-path">
          <span className="pw-strong">{fileName(s.path)}</span>
          <span className="pw-muted pw-small">{folderName(s.path)}</span>
        </span>
      </div>
      <div className="pw-import-body">
        {s.scene.paths.screenshot && (
          <img
            className="pw-import-thumb"
            src={s.scene.paths.screenshot}
            alt=""
            loading="lazy"
          />
        )}
        <div className="pw-grow pw-min0 pw-stack-sm">
          {isDVD && (
            <div className="pw-dvd-note">
              <Icon icon={faCompactDisc} />{" "}
              <FormattedMessage
                id="sports.dvd_title"
                values={{ duration: formatDuration(file?.duration) }}
              />
            </div>
          )}
          <div className="pw-fields">
            {s.fields.map((f) => (
              <span
                key={f.name}
                className={cx("pw-field", { "pw-field-guess": f.guessed })}
                title={
                  f.guessed
                    ? intl.formatMessage({ id: "sports.guessed" })
                    : intl.formatMessage({ id: `sports.origin.${f.origin}` })
                }
              >
                <small>
                  <FormattedMessage id={`sports.field.${f.name}`} />
                  {f.guessed && (
                    <span className="sr-only">
                      {" "}
                      ({intl.formatMessage({ id: "sports.guessed" })})
                    </span>
                  )}
                </small>
                <span>{f.value}</span>
              </span>
            ))}
            {s.fields.length === 0 && (
              <span className="pw-muted">
                <FormattedMessage id="sports.nothing_recognised" />
              </span>
            )}
          </div>
          {s.notes.map((n) => (
            <div key={n} className="pw-muted pw-small">
              {n}
            </div>
          ))}
        </div>
      </div>
      <div className="pw-import-actions">
        {choices.length > 0 && (
          <Form.Control
            as="select"
            className="input-control pw-choice"
            value={eventID}
            aria-label={intl.formatMessage({ id: "sports.choose_event" })}
            onChange={(e) => setEventID(e.currentTarget.value)}
          >
            {choices.map((e) => (
              <option key={e.id} value={e.id}>
                {eventLabel(e)}
                {e.round ? ` · R${e.round}` : ""}
              </option>
            ))}
          </Form.Control>
        )}
        <span className="pw-grow" />
        <Link
          className="btn btn-secondary"
          to={`/scenes/${s.scene.id}?tab=recording`}
        >
          <Icon icon={faPen} /> <FormattedMessage id="actions.edit" />
        </Link>
        <Button variant="secondary" disabled={dismissing} onClick={onDismiss}>
          <Icon icon={dismissed ? faRotateLeft : faEyeSlash} />{" "}
          <FormattedMessage
            id={dismissed ? "sports.restore" : "sports.dismiss"}
          />
        </Button>
        {!dismissed && (
          <Button
            variant="primary"
            disabled={!s.event || accepting}
            onClick={onAccept}
          >
            <Icon icon={faCheck} />{" "}
            <FormattedMessage id="sports.add_to_library" />
          </Button>
        )}
      </div>
    </article>
  );
};

const ScanButton: React.FC<{ onDone: () => void }> = ({ onDone }) => {
  const Toast = useToast();
  const [jobID, setJobID] = useState<string>();
  const onComplete = useCallback(
    (job?: JobFragment) => {
      setJobID(undefined);
      if (job?.status === GQL.JobStatus.Failed) Toast.error(job.error);
      onDone();
    },
    [Toast, onDone]
  );
  const { job } = useMonitorJob(jobID, onComplete);

  async function scan() {
    try {
      const r = await mutateMetadataScan({});
      setJobID(r.data?.metadataScan);
    } catch (e) {
      Toast.error(e);
    }
  }

  return (
    <div className="pw-sync">
      <Button variant="secondary" disabled={!!jobID} onClick={scan}>
        <Icon icon={faFolderOpen} />{" "}
        <FormattedMessage id="sports.scan_folders" />
      </Button>
      {jobID && (
        <div className="pw-sync-progress">
          <ProgressBar
            now={(job?.progress ?? 0) * 100}
            variant="warning"
            aria-label="progress"
          />
          <span className="pw-muted pw-ellipsis">
            {job?.subTasks?.[0] ?? job?.description}
          </span>
        </div>
      )}
    </div>
  );
};

export const ImportReview: React.FC = () => {
  const intl = useIntl();
  const Toast = useToast();
  const titleProps = useTitleProps({ id: "sports.import" });
  const [dismissed, setDismissed] = useState(false);
  const [page, setPage] = useState(1);

  const {
    data,
    loading,
    refetch: reload,
    previousData,
  } = GQL.useSportsImportSuggestionsQuery({
    variables: {
      input: { page, per_page: PER_PAGE, dismissed_only: dismissed },
    },
    fetchPolicy: "cache-and-network",
  });
  const [acceptAll, { loading: acceptingAll }] =
    GQL.useSportsImportAcceptAllMutation({ refetchQueries: refetch });

  const result = (data ?? previousData)?.sportsImportSuggestions;
  const items = result?.suggestions ?? [];
  const count = result?.count ?? 0;
  const pages = Math.max(1, Math.ceil(count / PER_PAGE));
  const high = items.filter(
    (s) => s.confidence === GQL.ImportConfidence.High
  ).length;

  async function onAcceptAll() {
    try {
      const r = await acceptAll({
        variables: { input: { min_confidence: GQL.ImportConfidence.High } },
      });
      Toast.success(
        intl.formatMessage(
          { id: "sports.accepted_n" },
          { count: r.data?.sportsImportAcceptAll ?? 0 }
        )
      );
      setPage(1);
    } catch (e) {
      Toast.error(e);
    }
  }

  return (
    <SportsLayout>
      <Helmet {...titleProps} />
      <header className="pw-page-head pw-page-head-end">
        <div className="pw-grow">
          <h1 className="pw-h1">
            <FormattedMessage id="sports.import" />
          </h1>
          <p className="pw-muted pw-mt-sm">
            <FormattedMessage
              id="sports.import_description"
              values={{ count }}
            />
          </p>
        </div>
        <div className="pw-actions">
          <ScanButton onDone={() => reload()} />
          {!dismissed && (
            <Button
              variant="primary"
              disabled={acceptingAll || count === 0}
              onClick={onAcceptAll}
            >
              <Icon icon={faCheck} />{" "}
              <FormattedMessage id="sports.accept_all_high" />
            </Button>
          )}
        </div>
      </header>

      <div className="pw-chips" role="tablist">
        <button
          type="button"
          role="tab"
          aria-selected={!dismissed}
          className={cx("pw-chip", { on: !dismissed })}
          onClick={() => {
            setDismissed(false);
            setPage(1);
          }}
        >
          <FormattedMessage id="sports.to_review" />
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={dismissed}
          className={cx("pw-chip", { on: dismissed })}
          onClick={() => {
            setDismissed(true);
            setPage(1);
          }}
        >
          <FormattedMessage id="sports.dismissed" />
        </button>
        {!dismissed && high > 0 && (
          <span className="pw-muted pw-small pw-ml-auto">
            <FormattedMessage
              id="sports.high_on_page"
              values={{ count: high }}
            />
          </span>
        )}
      </div>

      <div className="pw-columns">
        <div className="pw-wide">
          {loading && !result && <LoadingIndicator />}
          {result && items.length === 0 && (
            <EmptyState title={<FormattedMessage id="sports.import_empty" />}>
              <p className="pw-muted">
                <FormattedMessage id="sports.import_empty_description" />
              </p>
            </EmptyState>
          )}
          {items.length > 0 && (
            <Panel className="pw-import-list">
              {items.map((s) => (
                <SuggestionCard
                  key={s.scene.id}
                  s={s}
                  dismissed={dismissed}
                  onChanged={() => reload()}
                />
              ))}
            </Panel>
          )}
          {pages > 1 && (
            <nav
              className="pw-pager"
              aria-label={intl.formatMessage({ id: "sports.pages" })}
            >
              <Button
                variant="secondary"
                disabled={page <= 1}
                onClick={() => setPage(page - 1)}
              >
                <FormattedMessage id="sports.previous" />
              </Button>
              <span className="pw-mono pw-muted">
                {page} / {pages}
              </span>
              <Button
                variant="secondary"
                disabled={page >= pages}
                onClick={() => setPage(page + 1)}
              >
                <FormattedMessage id="sports.next" />
              </Button>
            </nav>
          )}
        </div>
        <aside className="pw-side">
          <Panel title={<FormattedMessage id="sports.how_matching_works" />}>
            <div className="pw-panel-sub pw-small">
              <p>
                <FormattedMessage id="sports.matching_explained" />
              </p>
              <ul className="pw-legend-list">
                <li>
                  <span className="pw-conf pw-conf-hi">
                    {intl.formatMessage({ id: "sports.confidence.HIGH" })}
                  </span>{" "}
                  <FormattedMessage id="sports.confidence_help.HIGH" />
                </li>
                <li>
                  <span className="pw-conf pw-conf-md">
                    {intl.formatMessage({ id: "sports.confidence.MEDIUM" })}
                  </span>{" "}
                  <FormattedMessage id="sports.confidence_help.MEDIUM" />
                </li>
                <li>
                  <span className="pw-conf pw-conf-lo">
                    {intl.formatMessage({ id: "sports.confidence.LOW" })}
                  </span>{" "}
                  <FormattedMessage id="sports.confidence_help.LOW" />
                </li>
                <li>
                  <span className="pw-conf pw-conf-none">
                    {intl.formatMessage({ id: "sports.confidence.NONE" })}
                  </span>{" "}
                  <FormattedMessage id="sports.confidence_help.NONE" />
                </li>
              </ul>
              <p className="pw-muted">
                <FormattedMessage
                  id="sports.naming_rules_help"
                  values={{
                    key: <code>sports_filename_patterns</code>,
                    example: (
                      <code>
                        {
                          "{series}.{year}.R{round}.{event}.{session}.{broadcaster}"
                        }
                      </code>
                    ),
                  }}
                />
              </p>
            </div>
          </Panel>
        </aside>
      </div>
    </SportsLayout>
  );
};

export default ImportReview;
