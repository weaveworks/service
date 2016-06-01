import React from 'react';
import { Route, IndexRoute } from 'react-router';

import OrganizationPage from './pages/organization/page';
import LandingPage from './pages/landing/page';
import CookieCheck from './pages/landing/cookie-check';
import Login from './pages/landing/login';
import LoginForm from './pages/landing/login-form';
import Logout from './pages/landing/logout';
import WrapperPage from './pages/wrapper/page';
import RouterComponent from './components/router';

export default function getRoutes() {
  return (
    <Route name="app" path="/" component={RouterComponent}>
      <Route name="wrapper" path="app/:orgId" component={WrapperPage} />
      <Route name="organization" path="org/:orgId" component={OrganizationPage} />
      <Route component={LandingPage}>
        <IndexRoute component={CookieCheck} />
        <Route name="login-form" path="login" component={LoginForm} />
        <Route name="login-form" path="login/:error" component={LoginForm} />
        <Route name="login" path="login/:email/:token" component={Login} />
        <Route name="logout" path="logout" component={Logout} />
      </Route>
    </Route>
  );
}
