import React from 'react';
import { HashLocation } from 'react-router';
import { getData } from '../../common/request';
import { CircularProgress, Styles } from 'material-ui';
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
    const params = this.props.params;
    const url = `/api/users/login?email=${params.email}&token=${params.token}`;
    getData(url).then(this._handleLoginSuccess, this._handleLoginError);
  }

  _handleLoginSuccess() {
    HashLocation.push('/');
  }

  _handleLoginError(resp) {
    if (resp.status === 401) {
      HashLocation.push('/login');
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
        opacity: 0.6,
        color: Styles.Colors.red900
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
