import React from "react";
import { Route, Switch } from "react-router-dom";
import { EventPage } from "./EventPage";
import { ImportReview } from "./ImportReview";
import { RecordingsBrowser } from "./RecordingsBrowser";
import { SeasonPage, SeasonsRedirect } from "./SeasonPage";

const SportsRoutes: React.FC = () => (
  <Switch>
    <Route exact path="/seasons" component={SeasonsRedirect} />
    <Route path="/series/:id/:season?" component={SeasonPage} />
    <Route path="/events/:id" component={EventPage} />
    <Route path="/recordings" component={RecordingsBrowser} />
    <Route path="/import" component={ImportReview} />
  </Switch>
);

export default SportsRoutes;
