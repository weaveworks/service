/**
 * App entry point
 */

// icon fonts, loaded via webpack, available via fa classes
require('font-awesome-webpack');

// Libraries
import React from 'react';
import ReactDOM from 'react-dom';

import getMuiTheme from 'material-ui/styles/getMuiTheme';
import MuiThemeProvider from 'material-ui/styles/MuiThemeProvider';

import { hashHistory, Router } from 'react-router';

// Routers
import getRoutes from './router';

// Tracking
import { trackTiming } from './common/tracking';

// ID of the DOM element to mount app on
const DOM_APP_EL_ID = 'app';

// Initialize routes
const routes = getRoutes();

ReactDOM.render(
  (<MuiThemeProvider muiTheme={getMuiTheme()}>
    <Router history={hashHistory}>{routes}</Router>
  </MuiThemeProvider>),
  document.getElementById(DOM_APP_EL_ID));

trackTiming('JS app', 'started');
