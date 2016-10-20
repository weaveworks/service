import React from 'react';
import { Redirect, Route, IndexRedirect, IndexRoute } from 'react-router';

import AccountPage from './pages/account/page';
import InstancesCreate from './pages/onboarding/instances-create';
import InstancesPage from './pages/instances/page';
import InstancesDeleted from './pages/onboarding/instances-deleted';
import InstancesError from './pages/onboarding/instances-error';
import InstancesPicker from './pages/onboarding/instances-picker';
import InstancesSelect from './pages/onboarding/instances-select';
import OnboardingPage from './pages/onboarding/page';
import OrganizationPage from './pages/organization/page';
import PromPage from './pages/prom/page';
import LandingPage from './pages/landing/page';
import CookieCheck from './pages/landing/cookie-check';
import Login from './pages/landing/login';
import LoginForm from './pages/landing/login-form';
import Logout from './pages/landing/logout';
import SignupForm from './pages/landing/signup-form';
import WrapperPage from './pages/wrapper/page';
import SettingsPage from './pages/settings/page';
import AccountSettings from './pages/settings/account/page';
import BillingSettings from './pages/settings/billing/page';
import RouterComponent from './components/router';

export default function getRoutes() {
  return (
    <Route name="app" path="/" component={RouterComponent}>

      {/* Logged in */}
      <Route name="wrapper" path="app/:orgId" component={WrapperPage} />
      <Route name="prom" path="prom/:orgId" component={PromPage} />
      <Route name="organization" path="org/:orgId" component={OrganizationPage} />
      <Route name="account" path="account/:orgId" component={AccountPage} />
      <Route name="instance" path="instance/:orgId" component={InstancesPage} />
      <Route path="instances" component={OnboardingPage}>
        <IndexRoute component={InstancesPicker} />
        <Route path="create(/:first)" component={InstancesCreate} />
        <Route path="select/:id" component={InstancesSelect} />
        <Route path="deleted" component={InstancesDeleted} />
        <Route path="error/:error" component={InstancesError} />
      </Route>
      <Route name="billing" path="settings/billing/:orgId" component={SettingsPage}>
        <Route path="*" component={BillingSettings} />
      </Route>
      <Route name="settings" path="settings/:orgId" component={SettingsPage}>
        <IndexRedirect to="account" />
        <Route path="account" component={AccountSettings} />
      </Route>

      {/* Sign up/Log in */}
      <Route component={LandingPage}>
        <IndexRoute component={CookieCheck} />
        <Redirect from="login/success" to="/" />
        <Redirect from="signup/success" to="/" />

        <Route name="register-form" path="signup" component={SignupForm} />
        <Route name="login-form" path="login(/:error)" component={LoginForm} />
        <Route name="login-worker-invite" path="login/:orgId/:email/:token" component={Login} />
        <Route name="login-worker" path="login/:email/:token" component={Login} />
        <Route name="login-via" path="login-via/:provider" component={Login} />
        <Route name="logout" path="logout" component={Logout} />
      </Route>
    </Route>
  );
}
