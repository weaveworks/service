import React from 'react';
import CircularProgress from 'material-ui/CircularProgress';
import RaisedButton from 'material-ui/RaisedButton';
import { red900 } from 'material-ui/styles/colors';
import { hashHistory } from 'react-router';

import { encodeURIs } from '../../common/request';
import { getOrganizations, getProbes } from '../../common/api';
import { trackException, trackView } from '../../common/tracking';

export default class CookieCheck extends React.Component {

  constructor(props) {
    super(props);

    this.state = {
      name: null,
      activityText: 'Checking login state...',
      errorText: ''
    };

    this._checkCookie = this._checkCookie.bind(this);
    this._handleLoginSuccess = this._handleLoginSuccess.bind(this);
    this._handleLoginError = this._handleLoginError.bind(this);
    this._handleClickReload = this._handleClickReload.bind(this);
    this.handleProbesSuccess = this.handleProbesSuccess.bind(this);
    this.handleProbesError = this.handleProbesError.bind(this);
  }

  componentDidMount() {
    this._checkCookie();
    trackView('CookieCheck');
  }

  _handleLoginError(resp) {
    if (resp.status === 401) {
      // if unauthorized, send to login page
      this.setState({
        activityText: 'Not logged in. Please wait for the login form to load...'
      });
      hashHistory.push('/signup');
    } else {
      const err = resp.errors[0];
      this.setState({
        activityText: '',
        errorText: err.message
      });
      trackException(err.message);
    }
  }

  _checkCookie() {
    getOrganizations().then(this._handleLoginSuccess, this._handleLoginError);
  }

  _handleLoginSuccess(resp) {
    if (resp.organizations.length >= 1) {
      const name = resp.organizations[0].name;
      getProbes(name).then(this.handleProbesSuccess, this.handleProbesError);
      this.setState({
        name,
        activityText: 'Logged in. Checking for connected probes...'
      });
    } else {
      const errorText = 'No team found. Please contact Weave Cloud support.';
      this.setState({
        activityText: '',
        errorText
      });
      trackException(errorText);
    }
  }

  handleProbesSuccess(resp) {
    const { name } = this.state;
    let url;
    if (resp && resp.length > 0) {
      // go to app if a probe is connected
      url = encodeURIs`/app/${name}`;
    } else {
      // otherwise go to management page
      url = encodeURIs`/org/${name}`;
    }
    hashHistory.push(url);
  }

  handleProbesError(resp) {
    const err = resp.errors[0];
    this.setState({
      activityText: '',
      errorText: err.message
    });
    trackException(err.message);
  }

  _handleClickReload() {
    window.location.reload(true);
  }

  render() {
    const styles = {
      error: {
        display: this.state.errorText ? 'block' : 'none',
        fontSize: '85%',
        opacity: 0.6,
        color: red900
      },

      activity: {
        textAlign: 'center',
        display: this.state.activityText ? 'block' : 'none',
        fontSize: '85%',
        opacity: 0.6
      }
    };

    return (
      <div>
        <div style={styles.activity}>
          <CircularProgress mode="indeterminate" />
          <p>{this.state.activityText}.</p>
        </div>
        <div style={styles.error}>
          <h3>Weave Cloud is not available. Please try again later.</h3>
          <p>{this.state.errorText}</p>
          <div>
            <RaisedButton onClick={this._handleClickReload}
              label="Try again" />
          </div>
        </div>
      </div>
    );
  }

}
