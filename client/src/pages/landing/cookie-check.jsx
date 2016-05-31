import React from 'react';
import { getData, encodeURIs } from '../../common/request';
import { CircularProgress } from 'material-ui';
import { red900 } from 'material-ui/styles/colors';
import { trackException, trackView } from '../../common/tracking';

export default class CookieCheck extends React.Component {

  constructor(props) {
    super(props);

    this.state = {
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
      this.props.history.push('/login');
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
    const url = `/api/users/lookup`;
    getData(url).then(this._handleLoginSuccess, this._handleLoginError);
  }

  _handleLoginSuccess(resp) {
    this.setState({
      activityText: 'Logged in. Please wait for your app to load...'
    });
    let url;
    if (resp.firstProbeUpdateAt) {
      // go to app if a probe is connected
      url = encodeURIs`/app/${resp.organizationName}`;
    } else {
      // otherwise go to management page
      url = encodeURIs`/org/${resp.organizationName}`;
    }
    this.props.history.push(url);
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
          <h3>Scope service is not available. Please try again later.</h3>
          <p>{this.state.errorText}</p>
        </div>
      </div>
    );
  }

}
