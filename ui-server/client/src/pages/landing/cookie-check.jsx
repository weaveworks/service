import React from 'react';
import CircularProgress from 'material-ui/CircularProgress';
import RaisedButton from 'material-ui/RaisedButton';
import { red900 } from 'material-ui/styles/colors';
import { browserHistory } from 'react-router';

import { encodeURIs } from '../../common/request';
import { getOrganizations } from '../../common/api';
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
      browserHistory.push('/signup');
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
    if (resp.organizations && resp.organizations.length > 1) {
      // choose instance
      browserHistory.push('/instances');
    } else if (resp.organizations && resp.organizations.length === 1) {
      // only one instance -> go straight there
      const id = resp.organizations[0].id;
      browserHistory.push(encodeURIs`/instances/select/${id}`);
    } else {
      browserHistory.push('/instances/create');
    }
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
