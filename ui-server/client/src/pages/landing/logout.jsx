import React from 'react';
import { hashHistory } from 'react-router';
import CircularProgress from 'material-ui/CircularProgress';
import { red900 } from 'material-ui/styles/colors';

import { getData } from '../../common/request';
import { trackException, trackView } from '../../common/tracking';

export default class Logout extends React.Component {

  constructor(props) {
    super(props);

    this.state = {
      activityText: 'Logging out...',
      errorText: ''
    };

    this._handleSuccess = this._handleSuccess.bind(this);
    this._handleError = this._handleError.bind(this);
  }

  componentDidMount() {
    this._tryLogout();
    trackView('Logout');
  }

  _tryLogout() {
    const url = '/api/users/logout';
    getData(url).then(this._handleSuccess, this._handleError);
  }

  _handleSuccess() {
    hashHistory.push('/');
  }

  _handleError(resp) {
    if (resp.status === 401) {
      // logout should not fail for Unauthorized
      hashHistory.push('/');
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
          <h3>Weave Cloud is not available. Please try again later.</h3>
          <p>{this.state.errorText}</p>
        </div>
      </div>
    );
  }

}
