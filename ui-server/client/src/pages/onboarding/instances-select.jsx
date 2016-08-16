import React from 'react';
import CircularProgress from 'material-ui/CircularProgress';
import RaisedButton from 'material-ui/RaisedButton';
import { red900 } from 'material-ui/styles/colors';
import { browserHistory } from 'react-router';

import { encodeURIs } from '../../common/request';
import { getProbes } from '../../common/api';
import { trackException, trackView } from '../../common/tracking';

export default class InstancesSelect extends React.Component {

  constructor(props) {
    super(props);

    this.state = {
      activityText: 'Checking for connected probes...',
      errorText: ''
    };

    this.handleProbesSuccess = this.handleProbesSuccess.bind(this);
    this.handleProbesError = this.handleProbesError.bind(this);
  }

  componentDidMount() {
    this.checkProbes();
    trackView('InstanceSelect');
  }

  componentWillUnmount() {
    this.mounted = false;
  }

  checkProbes() {
    const { id } = this.props.params;
    if (id) {
      getProbes(id).then(this.handleProbesSuccess, this.handleProbesError);
      this.mounted = true;
    } else {
      const errorText = 'Need instance ID to proceed.';
      this.setState({ errorText });
      trackException(errorText);
    }
  }

  handleProbesSuccess(resp) {
    const { id } = this.props.params;
    let url;
    if (resp && resp.length > 0) {
      // go to app if a probe is connected
      url = encodeURIs`/app/${id}`;
    } else {
      // otherwise go to management page
      url = encodeURIs`/org/${id}`;
    }
    if (this.mounted) {
      browserHistory.push(url);
    }
  }

  handleProbesError() {
    // go to management page if we failed to get the probes
    const { id } = this.props.params;
    const url = encodeURIs`/org/${id}`;
    if (this.mounted) {
      browserHistory.push(url);
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
