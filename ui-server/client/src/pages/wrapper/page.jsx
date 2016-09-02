/* eslint react/jsx-no-bind:0 */
import React from 'react';
import ReactDOM from 'react-dom';
import CircularProgress from 'material-ui/CircularProgress';

import { getData, encodeURIs } from '../../common/request';
import PrivatePage from '../../components/private-page';
import { trackView } from '../../common/tracking';

export default class Wrapper extends React.Component {

  constructor() {
    super();

    this.state = {
      activityText: '',
      frameBaseUrl: ''
    };

    this._checkInstance = this._checkInstance.bind(this);
    this._handleInstanceError = this._handleInstanceError.bind(this);
    this._handleInstanceSuccess = this._handleInstanceSuccess.bind(this);
    this.frameState = window.location.hash;
  }

  componentDidMount() {
    // check if scope instance is ready
    this._checkInstance();
    // track internal view state
    this.startFrameUrlTracker();
    trackView('Wrapper');
  }

  componentWillReceiveProps(nextProps) {
    if (nextProps.params.orgId !== this.props.params.orgId) {
      this._checkInstance();
    }
  }

  startFrameUrlTracker() {
    // periodically check iframe's URL and react to changes
    clearInterval(this.frameStateChecker);

    this.frameStateChecker = setInterval(() => {
      const iframe = ReactDOM.findDOMNode(this._iframe);
      if (iframe) {
        const target = iframe.contentWindow;
        if (this.frameState !== target.location.hash) {
          this._onFrameStateChanged(target.location.hash);
        }
      }
    }, 1000);
  }

  componentWillUnmount() {
    clearInterval(this.frameStateChecker);
  }

  _onFrameStateChanged(nextFrameState) {
    window.location.hash = nextFrameState;
    this.frameState = nextFrameState;
  }

  _checkInstance() {
    const url = encodeURIs`/api/app/${this.props.params.orgId}/api`;
    getData(url).then(this._handleInstanceSuccess, this._handleInstanceError);
  }

  _handleInstanceSuccess() {
    let url = encodeURIs`/api/app/${this.props.params.orgId}/`;
    if (this.frameState) {
      url = `${url}${this.frameState}`;
    }
    this.setState({
      activityText: '',
      frameBaseUrl: url
    });
  }

  _handleInstanceError(resp) {
    if (resp.status === 503) {
      // not ready, try again
      this.setState({
        activityText: 'Spawning your Weave Cloud instance...'
      });
    } else {
      this.setState({
        activityText: `Error while checking for your Weave Cloud instance. [${resp.status}]`
      });
    }
    setTimeout(this._checkInstance, 2000);
  }

  shouldComponentUpdate(nextProps, nextState) {
    return this.state.frameBaseUrl !== nextState.frameBaseUrl;
  }

  render() {
    const styles = {
      activity: {
        marginTop: 200,
        textAlign: 'center'
      },
      iframe: {
        display: 'block',
        border: 'none',
        height: 'calc(100vh - 56px)',
        width: '100%'
      }
    };

    // forward wrapper state to scope UI via src URL
    const frameUrl = `${this.state.frameBaseUrl}`;

    return (
      <PrivatePage page="wrapper" {...this.props.params}>
        {this.state.activityText && <div>
          <div style={styles.activity}>
            <p>{this.state.activityText}</p>
            <CircularProgress mode="indeterminate" />
          </div>
        </div>}
        {this.state.frameBaseUrl && <iframe ref={(c) => {this._iframe = c;}}
          onLoad={this._handleFrameLoad} src={frameUrl} style={styles.iframe} />}
      </PrivatePage>
    );
  }
}
