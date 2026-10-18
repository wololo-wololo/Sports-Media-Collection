import React, { useMemo, useState } from "react";
import { Button } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { NavLink } from "react-router-dom";
import cx from "classnames";
import { faPlus } from "@fortawesome/free-solid-svg-icons";
import * as GQL from "src/core/generated-graphql";
import { Icon } from "src/components/Shared/Icon";
import { SeriesDot } from "./common";
import { SeriesDialog } from "./SeriesDialog";

const SeriesRail: React.FC<{ activeSeriesID?: string }> = ({
  activeSeriesID,
}) => {
  const intl = useIntl();
  const { data } = GQL.useAllSeriesQuery();
  const [adding, setAdding] = useState(false);

  const bySport = useMemo(() => {
    const groups = new Map<string, GQL.SeriesDataFragment[]>();
    for (const s of data?.allSeries ?? []) {
      const list = groups.get(s.sport) ?? [];
      list.push(s);
      groups.set(s.sport, list);
    }
    // motorsport first, then the other sports by name
    return [...groups.entries()].sort(([a], [b]) =>
      a === "Motorsport" ? -1 : b === "Motorsport" ? 1 : a.localeCompare(b)
    );
  }, [data]);

  return (
    <nav
      className="pw-rail"
      aria-label={intl.formatMessage({ id: "sports.series" })}
    >
      {bySport.map(([sport, series]) => (
        <div key={sport} className="pw-rail-group">
          <div className="pw-rail-label">{sport}</div>
          {series.map((s) => (
            <NavLink
              key={s.id}
              to={`/series/${s.id}`}
              className={cx("pw-rail-item", {
                active: s.id === activeSeriesID,
              })}
            >
              <SeriesDot color={s.color} />
              <span className="pw-rail-name">{s.name}</span>
              <span className="pw-rail-count">
                {intl.formatNumber(s.scene_count)}
              </span>
            </NavLink>
          ))}
        </div>
      ))}
      <Button
        variant="link"
        className="pw-rail-item pw-rail-add"
        onClick={() => setAdding(true)}
      >
        <Icon icon={faPlus} /> <FormattedMessage id="sports.add_series" />
      </Button>
      {adding && <SeriesDialog onClose={() => setAdding(false)} />}
    </nav>
  );
};

// Page frame of the sports screens: the series rail beside the content.
export const SportsLayout: React.FC<
  React.PropsWithChildren<{ activeSeriesID?: string; className?: string }>
> = ({ activeSeriesID, className, children }) => (
  <div className={cx("pw-layout", className)}>
    <SeriesRail activeSeriesID={activeSeriesID} />
    <main className="pw-main">{children}</main>
  </div>
);
