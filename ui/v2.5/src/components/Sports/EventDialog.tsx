import React, { useState } from "react";
import { Col, Form, Row } from "react-bootstrap";
import { useIntl } from "react-intl";
import { faCalendarPlus } from "@fortawesome/free-solid-svg-icons";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import { sessionMessageID, sessionTypeOrder } from "./utils";

interface IEventDialog {
  seriesID: string;
  season: number;
  event?: GQL.EventDataFragment;
  onClose: (saved?: GQL.EventDataFragment) => void;
}

const refetch = [
  "SeasonCoverage",
  "SeriesCoverage",
  "AllSeries",
  "FindEvent",
  "FindSeries",
];

export const EventDialog: React.FC<IEventDialog> = ({
  seriesID,
  season,
  event,
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const [create] = GQL.useEventCreateMutation({ refetchQueries: refetch });
  const [update] = GQL.useEventUpdateMutation({ refetchQueries: refetch });

  const [values, setValues] = useState({
    name: event?.name ?? "",
    season: String(event?.season ?? season),
    round: event?.round ? String(event.round) : "",
    start_date: event?.start_date ?? "",
    end_date: event?.end_date ?? "",
    circuit: event?.circuit ?? "",
    location: event?.location ?? "",
    country: event?.country ?? "",
  });
  const [saving, setSaving] = useState(false);

  const set =
    (k: keyof typeof values) => (e: React.ChangeEvent<HTMLInputElement>) =>
      setValues({ ...values, [k]: e.currentTarget.value });

  const seasonNum = Number.parseInt(values.season, 10);
  const valid =
    values.name.trim() !== "" && seasonNum >= 1900 && seasonNum <= 2200;

  async function save() {
    setSaving(true);
    const input = {
      name: values.name.trim(),
      season: seasonNum,
      round: values.round ? Number.parseInt(values.round, 10) : null,
      start_date: values.start_date || null,
      end_date: values.end_date || null,
      circuit: values.circuit.trim() || null,
      location: values.location.trim() || null,
      country: values.country.trim() || null,
    };
    try {
      if (event) {
        const r = await update({
          variables: { input: { id: event.id, ...input } },
        });
        onClose(r.data?.eventUpdate ?? undefined);
      } else {
        const r = await create({
          variables: { input: { series_id: seriesID, ...input } },
        });
        onClose(r.data?.eventCreate ?? undefined);
      }
    } catch (e) {
      Toast.error(e);
      setSaving(false);
    }
  }

  const field = (
    k: keyof typeof values,
    label: string,
    type = "text",
    extra?: Pick<
      React.InputHTMLAttributes<HTMLInputElement>,
      "min" | "max" | "step" | "placeholder" | "required" | "autoFocus"
    >
  ) => (
    <Form.Group controlId={`event-${k}`}>
      <Form.Label>{intl.formatMessage({ id: label })}</Form.Label>
      <Form.Control
        className="text-input"
        type={type}
        value={values[k]}
        onChange={set(k)}
        {...extra}
      />
    </Form.Group>
  );

  return (
    <ModalComponent
      show
      icon={faCalendarPlus}
      header={intl.formatMessage({
        id: event ? "sports.edit_event" : "sports.add_event",
      })}
      accept={{
        text: intl.formatMessage({ id: "actions.save" }),
        onClick: save,
      }}
      cancel={{
        text: intl.formatMessage({ id: "actions.cancel" }),
        variant: "secondary",
        onClick: () => onClose(),
      }}
      disabled={!valid}
      isRunning={saving}
      onHide={() => onClose()}
    >
      <Form
        onSubmit={(e) => {
          e.preventDefault();
          if (valid) save();
        }}
      >
        {field("name", "sports.event_name", "text", { autoFocus: true })}
        <Row>
          <Col xs={6}>
            {field("season", "sports.season", "number", {
              min: 1900,
              max: 2200,
            })}
          </Col>
          <Col xs={6}>
            {field("round", "sports.round", "number", { min: 1, max: 99 })}
          </Col>
        </Row>
        <Row>
          <Col xs={6}>{field("start_date", "sports.start_date", "date")}</Col>
          <Col xs={6}>{field("end_date", "sports.end_date", "date")}</Col>
        </Row>
        {field("circuit", "sports.circuit")}
        <Row>
          <Col xs={6}>{field("location", "sports.location")}</Col>
          <Col xs={6}>{field("country", "sports.country")}</Col>
        </Row>
      </Form>
    </ModalComponent>
  );
};

interface ISessionDialog {
  eventID: string;
  session?: GQL.EventSessionDataFragment;
  onClose: () => void;
}

// local datetime-input value for an ISO timestamp
function toLocalInput(iso?: string | null) {
  if (!iso) return "";
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export const SessionDialog: React.FC<ISessionDialog> = ({
  eventID,
  session,
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const [create] = GQL.useEventSessionCreateMutation({
    refetchQueries: refetch,
  });
  const [update] = GQL.useEventSessionUpdateMutation({
    refetchQueries: refetch,
  });

  const [type, setType] = useState<GQL.SessionType>(
    session?.session_type ?? GQL.SessionType.Race
  );
  const [name, setName] = useState(session?.name ?? "");
  const [start, setStart] = useState(toLocalInput(session?.start_time));
  const [winner, setWinner] = useState(session?.winner ?? "");
  const [saving, setSaving] = useState(false);

  async function save() {
    setSaving(true);
    const input = {
      session_type: type,
      name: name.trim() || null,
      start_time: start ? new Date(start).toISOString() : null,
      winner: winner.trim() || null,
    };
    try {
      if (session) {
        await update({ variables: { input: { id: session.id, ...input } } });
      } else {
        await create({ variables: { input: { event_id: eventID, ...input } } });
      }
      onClose();
    } catch (e) {
      Toast.error(e);
      setSaving(false);
    }
  }

  return (
    <ModalComponent
      show
      icon={faCalendarPlus}
      header={intl.formatMessage({
        id: session ? "sports.edit_session" : "sports.add_session",
      })}
      accept={{
        text: intl.formatMessage({ id: "actions.save" }),
        onClick: save,
      }}
      cancel={{
        text: intl.formatMessage({ id: "actions.cancel" }),
        variant: "secondary",
        onClick: () => onClose(),
      }}
      isRunning={saving}
      onHide={() => onClose()}
    >
      <Form
        onSubmit={(e) => {
          e.preventDefault();
          save();
        }}
      >
        <Form.Group controlId="session-type">
          <Form.Label>
            {intl.formatMessage({ id: "sports.session" })}
          </Form.Label>
          <Form.Control
            as="select"
            className="input-control"
            value={type}
            onChange={(e) => setType(e.currentTarget.value as GQL.SessionType)}
          >
            {sessionTypeOrder.map((t) => (
              <option key={t} value={t}>
                {intl.formatMessage({ id: sessionMessageID(t) })}
              </option>
            ))}
          </Form.Control>
        </Form.Group>
        <Form.Group controlId="session-name">
          <Form.Label>
            {intl.formatMessage({ id: "sports.session_name" })}
          </Form.Label>
          <Form.Control
            className="text-input"
            value={name}
            placeholder={intl.formatMessage({ id: "sports.optional" })}
            onChange={(e) => setName(e.currentTarget.value)}
          />
        </Form.Group>
        <Form.Group controlId="session-start">
          <Form.Label>
            {intl.formatMessage({ id: "sports.start_time" })}
          </Form.Label>
          <Form.Control
            className="text-input"
            type="datetime-local"
            value={start}
            onChange={(e) => setStart(e.currentTarget.value)}
          />
        </Form.Group>
        <Form.Group controlId="session-winner">
          <Form.Label>{intl.formatMessage({ id: "sports.winner" })}</Form.Label>
          <Form.Control
            className="text-input"
            value={winner}
            onChange={(e) => setWinner(e.currentTarget.value)}
          />
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};
