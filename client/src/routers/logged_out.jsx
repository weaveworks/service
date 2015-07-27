import React from "react";
import { Route, DefaultRoute, RouteHandler } from "react-router";

import OrganizationPage from "../pages/organization/page";
import LandingPage from "../pages/landing/page";


export default class LoggedOutRouter extends React.Component {
  render() {
    return (
      <div id="container">
        <div id="main">
          <RouteHandler {...this.props} />
        </div>
      </div>
    );
  }

  static getRoutes = function() {
    return (
      <Route name="app" path="/" handler={LoggedOutRouter}>
        <Route name="organization" path="/org/:orgId" handler={OrganizationPage} />
        <DefaultRoute name="landing" handler={LandingPage} />
      </Route>
    );
  }
}
