import React from 'react';
import { hashHistory } from 'react-router';

import { getData } from '../../common/request';
import { CircularProgress } from 'material-ui';
import { red900 } from 'material-ui/styles/colors';
import { trackException, trackView } from '../../common/tracking';

export default class LoginForm extends React.Component {

  constructor(props) {
    super(props);

    this.state = {
      activityText: 'Logging in...',
      errorText: ''
    };

    this._handleLoginSuccess = this._handleLoginSuccess.bind(this);
    this._handleLoginError = this._handleLoginError.bind(this);
  }

  componentDidMount() {
    // triggered on fresh page load with login params
    this._tryLogin();
    trackView('Login');
  }

  _tryLogin() {
    let url = '/api/users/login';
    if (this.props.params.provider) {
      url += `/${this.props.params.provider}`;
    }
    getData(url, Object.assign({}, this.props.params, this.props.location.query))
      .then(this._handleLoginSuccess, this._handleLoginError);
  }

  _handleLoginSuccess(resp) {
    if (resp.redirectTo) {
      hashHistory.push(resp.redirectTo);
    } else {
      hashHistory.push('/');
    }
  }

  _handleLoginError(resp) {
    if (resp.status === 401) {
      trackException('Server returned Unauthorized for login link');
      hashHistory.push('/login/unauthorized');
    } else {
      this.setState({
        activityText: '',
        errorText: resp.errors[0].message
      });
      trackException(resp.errors[0].message);
    }
  }

  render() {
    const styles = {
      error: {
        display: this.state.errorText ? 'block' : 'none',
        fontSize: '85%',
        color: red900
      },

      activity: {
        textAlign: 'center',
        display: this.state.activityText ? 'block' : 'none',
        fontSize: '85%',
        opacity: 0.8
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
