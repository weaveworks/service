import React from "react";
import { Route, DefaultRoute, RouteHandler } from "react-router";

import OrganizationPage from "./pages/organization/page";
import LandingPage from "./pages/landing/page";
import CookieCheck from "./pages/landing/cookie-check";
import Login from "./pages/landing/login";
import LoginForm from "./pages/landing/login-form";
import Logout from "./pages/landing/logout";
import WrapperPage from "./pages/wrapper/page";


export default class RouterComponent extends React.Component {
  render() {
    return (
      <div id="container" style={{height: '100%'}}>
        <RouteHandler {...this.props} />
      </div>
    );
  }

  static getRoutes = function() {
    return (
      <Route name="app" path="/" handler={RouterComponent}>
        <Route name="wrapper" path="app/:orgId" handler={WrapperPage} />
        <Route name="organization" path="org/:orgId" handler={OrganizationPage} />
        <Route handler={LandingPage}>
          <Route name="login-form" path="login" handler={LoginForm} />
          <Route name="login" path="login/:email/:token" handler={Login} />
          <Route name="logout" path="logout" handler={Logout} />
          <DefaultRoute handler={CookieCheck} />
        </Route>
      </Route>
    );
  }
}
