import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { useIntl } from "react-intl";
import { faFlagCheckered } from "@fortawesome/free-solid-svg-icons";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";

const swatches = [
  "#E8473C",
  "#3A8DFF",
  "#FF8A00",
  "#2FD07A",
  "#FFD23F",
  "#B36BFF",
  "#8C96A3",
];

interface ISeriesDialog {
  series?: GQL.SeriesDataFragment;
  onClose: (saved?: GQL.SeriesDataFragment) => void;
}

export const SeriesDialog: React.FC<ISeriesDialog> = ({ series, onClose }) => {
  const intl = useIntl();
  const Toast = useToast();
  const [create] = GQL.useSeriesCreateMutation({
    refetchQueries: ["AllSeries"],
  });
  const [update] = GQL.useSeriesUpdateMutation({
    refetchQueries: ["AllSeries"],
  });

  const [name, setName] = useState(series?.name ?? "");
  const [shortName, setShortName] = useState(series?.short_name ?? "");
  const [sport, setSport] = useState(series?.sport ?? "Motorsport");
  const [color, setColor] = useState(series?.color ?? swatches[1]);
  const [saving, setSaving] = useState(false);

  async function save() {
    setSaving(true);
    try {
      const input = {
        name: name.trim(),
        short_name: shortName.trim() || null,
        sport: sport.trim() || null,
        color,
      };
      if (series) {
        const r = await update({
          variables: { input: { id: series.id, ...input } },
        });
        onClose(r.data?.seriesUpdate ?? undefined);
      } else {
        const r = await create({ variables: { input } });
        onClose(r.data?.seriesCreate ?? undefined);
      }
    } catch (e) {
      Toast.error(e);
      setSaving(false);
    }
  }

  return (
    <ModalComponent
      show
      icon={faFlagCheckered}
      header={intl.formatMessage({
        id: series ? "sports.edit_series" : "sports.add_series",
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
      disabled={!name.trim()}
      isRunning={saving}
      onHide={() => onClose()}
    >
      <Form
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim()) save();
        }}
      >
        <Form.Group controlId="series-name">
          <Form.Label>
            {intl.formatMessage({ id: "sports.series_name" })}
          </Form.Label>
          <Form.Control
            className="text-input"
            value={name}
            placeholder="MotoGP"
            autoFocus
            onChange={(e) => setName(e.currentTarget.value)}
          />
        </Form.Group>
        <Form.Group controlId="series-short-name">
          <Form.Label>
            {intl.formatMessage({ id: "sports.short_name" })}
          </Form.Label>
          <Form.Control
            className="text-input"
            value={shortName}
            placeholder="MGP"
            onChange={(e) => setShortName(e.currentTarget.value)}
          />
        </Form.Group>
        <Form.Group controlId="series-sport">
          <Form.Label>{intl.formatMessage({ id: "sports.sport" })}</Form.Label>
          <Form.Control
            className="text-input"
            value={sport}
            list="sports-list"
            onChange={(e) => setSport(e.currentTarget.value)}
          />
          <datalist id="sports-list">
            {["Motorsport", "Football", "Tennis", "Cycling", "Athletics"].map(
              (s) => (
                <option key={s} value={s} />
              )
            )}
          </datalist>
        </Form.Group>
        <fieldset>
          <legend className="form-label">
            {intl.formatMessage({ id: "sports.colour" })}
          </legend>
          <div className="pw-swatches">
            {swatches.map((c) => (
              <button
                type="button"
                key={c}
                className={c === color ? "active" : ""}
                style={{ background: c }}
                aria-label={c}
                aria-pressed={c === color}
                onClick={() => setColor(c)}
              />
            ))}
          </div>
        </fieldset>
      </Form>
    </ModalComponent>
  );
};
