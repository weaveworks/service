import React from "react";
import { Styles } from "material-ui";
import { HashLocation } from "react-router";

import { getData } from "../../common/request";
import { Container } from "../../components/container";
import Toolbar from "../../components/toolbar";

const ThemeManager = new Styles.ThemeManager();

export default class Wrapper extends React.Component {

  static childContextTypes = {
    muiTheme: React.PropTypes.object
  }

  constructor() {
    super();

    this.state = {
      activityText: '',
      frameBaseUrl: ''
    };

    this._checkCookie = this._checkCookie.bind(this);
    this._handleLoginSuccess = this._handleLoginSuccess.bind(this);
    this._handleLoginError = this._handleLoginError.bind(this);
  }

  getChildContext() {
    return {
      muiTheme: ThemeManager.getCurrentTheme()
    };
  }

  componentDidUpdate() {
    if (this.refs.iframe) {
      // periodically check iframe's URL and react to changes
      clearInterval(this.frameStateChecker);
      let target = this.refs.iframe.getDOMNode().contentWindow;

      this.frameStateChecker = setInterval(() => {
        if (this.frameState !== target.location.hash) {
          this.frameState = target.location.hash;
          this._onFrameStateChanged(this.frameState);
        }
      }.bind(this), 1000);
    }

  }

  componentDidMount() {
    // check if we're logged in
    this._checkCookie();
  }

  componentWillUnmount() {
    clearInterval(this.frameStateChecker);
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
        {this.state.frameBaseUrl && <iframe ref="iframe"
          onLoad={this._handleFrameLoad.bind(this)} src={frameUrl} style={styles.iframe} />}
      </div>
    );
  }

  _handleFrameLoad(e) {
    console.log(e);
  }

  _onFrameStateChanged(hash) {
  }

  _checkCookie() {
    const url = `/api/users/org/${this.props.params.orgId}`;
    getData(url).then(this._handleLoginSuccess, this._handleLoginError);
  }

  _handleLoginSuccess(resp) {
    const url = `/api/app/${resp.name}`;
    this.setState({
      user: resp.user,
      frameBaseUrl: url
    });
  }

  _handleLoginError(resp) {
    if (resp.status === 401) {
      // if unauthorized, send to login page
      this.setState({
        activityText: 'Not logged in. Please wait for the login form to load...'
      });
      HashLocation.push('/login');
    } else {
      let err = resp.errors[0];
      console.error(err);
      this.setState({
        activityText: '',
        errorText: err.message
      });
    }
  }


}
