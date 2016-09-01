/**
 * App entry point
 */

// Polyfills for promises.
import 'babel-polyfill';

// icon fonts, loaded via webpack, available via fa classes
require('font-awesome-webpack');

// Required by material-ui. Their bigger components only listen to touch (taps).
// This sets up some clicks->taps voodoo.
const injectTapEventPlugin = require('react-tap-event-plugin');
injectTapEventPlugin();

// Libraries
import React from 'react';
import ReactDOM from 'react-dom';

import getMuiTheme from 'material-ui/styles/getMuiTheme';
import MuiThemeProvider from 'material-ui/styles/MuiThemeProvider';

import { browserHistory, Router } from 'react-router';

import { Provider } from 'react-redux';

// Routers
import getRoutes from './router';

// Tracking
import { generateSessionCookie, trackTiming } from './common/tracking';

import configureStore from './stores/configureStore';

// ID of the DOM element to mount app on
const DOM_APP_EL_ID = 'app';

// make our own tracking unique
document.cookie = generateSessionCookie();

// Initialize routes
const routes = getRoutes();

const store = configureStore();

ReactDOM.render(
  (<MuiThemeProvider muiTheme={getMuiTheme()}>
     <Provider store={store}>
       <Router history={browserHistory}>{routes}</Router>
    </Provider>
  </MuiThemeProvider>),
  document.getElementById(DOM_APP_EL_ID));

trackTiming('JS app', 'started');
