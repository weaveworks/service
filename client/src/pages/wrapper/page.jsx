import React from 'react';
import { CircularProgress } from 'material-ui';
import debug from 'debug';
import ThemeManager from 'material-ui/lib/styles/theme-manager';
import LightRawTheme from 'material-ui/lib/styles/raw-themes/light-raw-theme';

import { getData } from '../../common/request';
import Toolbar from '../../components/toolbar';
import { trackView } from '../../common/tracking';

const log = debug('service:wrapper');

export default class Wrapper extends React.Component {

  constructor() {
    super();

    this.state = {
      muiTheme: ThemeManager.getMuiTheme(LightRawTheme),
      activityText: '',
      frameBaseUrl: '',
      name: '',
      user: ''
    };

    this._checkCookie = this._checkCookie.bind(this);
    this._checkInstance = this._checkInstance.bind(this);
    this._handleInstanceError = this._handleInstanceError.bind(this);
    this._handleInstanceSuccess = this._handleInstanceSuccess.bind(this);
    this._handleLoginSuccess = this._handleLoginSuccess.bind(this);
    this._handleLoginError = this._handleLoginError.bind(this);
  }

  getChildContext() {
    return {
      muiTheme: this.state.muiTheme
    };
  }

  componentDidMount() {
    // check if we're logged in
    this._checkCookie();
    trackView('Wrapper');
  }

  componentDidUpdate() {
    if (this.refs.iframe) {
      // periodically check iframe's URL and react to changes
      clearInterval(this.frameStateChecker);
      const target = this.refs.iframe.getDOMNode().contentWindow;

      this.frameStateChecker = setInterval(() => {
        if (this.frameState !== target.location.hash) {
          this.frameState = target.location.hash;
          this._onFrameStateChanged(this.frameState);
        }
      }, 1000);
    }
  }

  componentWillUnmount() {
    clearInterval(this.frameStateChecker);
  }

  _handleFrameLoad(err) {
    log(err);
  }

  _onFrameStateChanged() {
  }

  _checkCookie() {
    const url = `/api/users/org/${this.props.params.orgId}`;
    getData(url).then(this._handleLoginSuccess, this._handleLoginError);
  }

  _checkInstance(name) {
    const org = name || this.state.name;
    const url = `/api/app/${org}/api`;
    getData(url).then(this._handleInstanceSuccess, this._handleInstanceError);
  }

  _handleLoginSuccess(resp) {
    this.setState({
      user: resp.user,
      name: resp.name
    });
    // check if scope instance is ready
    this._checkInstance(resp.name);
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
      log(err);
      this.setState({
        activityText: '',
        errorText: err.message
      });
    }
  }

  _handleInstanceSuccess() {
    const url = `/api/app/${this.state.name}/`;
    this.setState({
      activityText: '',
      frameBaseUrl: url
    });
  }

  _handleInstanceError(resp) {
    if (resp.status === 503) {
      // not ready, try again
      this.setState({
        activityText: 'Spawning your Scope app...'
      });
    } else {
      this.setState({
        activityText: `Error while checking for your Scope instance. [${resp.status}]`
      });
    }
    setTimeout(this._checkInstance, 2000);
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
        width: '100vw'
      }
    };

    // forward wrapper state to scope UI via src URL
    const frameUrl = `${this.state.frameBaseUrl}/${location.hash}`;

    return (
      <div>
        <Toolbar organization={this.props.params.orgId} user={this.state.user} />
        {this.state.activityText && <div>
          <div style={styles.activity}>
            <p>{this.state.activityText}</p>
            <CircularProgress mode="indeterminate" />
          </div>
        </div>}
        {this.state.frameBaseUrl && <iframe ref="iframe"
          onLoad={this._handleFrameLoad.bind(this)} src={frameUrl} style={styles.iframe} />}
      </div>
    );
  }
}

Wrapper.childContextTypes = {
  muiTheme: React.PropTypes.object
};
