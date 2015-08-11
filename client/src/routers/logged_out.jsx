import React from "react";
import { Route, DefaultRoute, RouteHandler } from "react-router";

import OrganizationPage from "../pages/organization/page";
import LandingPage from "../pages/landing/page";
import LoginPage from "../pages/login/page";
import WrapperPage from "../pages/wrapper/page";


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
        <Route name="wrapper" path="app/:orgId" handler={WrapperPage} />
        <Route name="organization" path="org/:orgId" handler={OrganizationPage} />
        <Route name="login" path="login" handler={LoginPage} />
        <Route name="login-form" path="login/:email/:token" handler={LoginPage} />
        <DefaultRoute name="landing" handler={LandingPage} />
      </Route>
    );
  }
}
