import React from 'react';
import { Redirect, Route, IndexRoute } from 'react-router';

import AccountPage from './pages/account/page';
import InstancesCreate from './pages/instances/instances-create';
import InstancesPicker from './pages/instances/instances-picker';
import InstancesPage from './pages/instances/page';
import InstancesSelect from './pages/instances/instances-select';
import OrganizationPage from './pages/organization/page';
import LandingPage from './pages/landing/page';
import CookieCheck from './pages/landing/cookie-check';
import Login from './pages/landing/login';
import LoginForm from './pages/landing/login-form';
import Logout from './pages/landing/logout';
import SignupForm from './pages/landing/signup-form';
import WrapperPage from './pages/wrapper/page';
import RouterComponent from './components/router';

export default function getRoutes() {
  return (
    <Route name="app" path="/" component={RouterComponent}>

      {/* Logged in */}
      <Route name="wrapper" path="app/:orgId" component={WrapperPage} />
      <Route name="organization" path="org/:orgId" component={OrganizationPage} />
      <Route name="account" path="account/:orgId" component={AccountPage} />
      <Route path="instances" component={InstancesPage}>
        <IndexRoute component={InstancesPicker} />
        <Route path="create" component={InstancesCreate} />
        <Route path="select/:name" component={InstancesSelect} />
      </Route>

      {/* Sign up/Log in */}
      <Route component={LandingPage}>
        <IndexRoute component={CookieCheck} />
        <Redirect from="login/success" to="/" />
        <Redirect from="signup/success" to="/" />

        <Route name="register-form" path="signup" component={SignupForm} />
        <Route name="login-form" path="login(/:error)" component={LoginForm} />
        <Route name="login-worker" path="login/:email/:token" component={Login} />
        <Route name="login-via" path="login-via/:provider" component={Login} />
        <Route name="logout" path="logout" component={Logout} />
      </Route>
    </Route>
  );
}
