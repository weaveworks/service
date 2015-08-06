import React from "react";
import page from "page";
import { Styles } from "material-ui";

import { getData } from "../../common/request";
import { Container } from "../../components/container";
import WrapperToolbar from "./toolbar";

const ThemeManager = new Styles.ThemeManager();

export default class Wrapper extends React.Component {

  static childContextTypes = {
    muiTheme: React.PropTypes.object
  }

  getChildContext() {
    return {
      muiTheme: ThemeManager.getCurrentTheme()
    };
  }

  componentDidMount() {
    // periodically check iframe's URL and react to changes
    let target = this.refs.iframe.getDOMNode().contentWindow;

    this.frameStateChecker = setInterval(() => {
      if (this.frameState !== target.location.hash) {
        this.frameState = target.location.hash;
        this._onFrameStateChanged(this.frameState);
      }
    }.bind(this), 1000);

    // initialize UI state tracking
    page.start({hashbang: true});
  }

  componentWillUnmount() {
    clearInterval(this.frameStateChecker);
    page.stop();
  }

  render() {

    const styles = {
      iframe: {
        display: 'block',
        border: 'none',
        height: 'calc(100vh - 56px)',
        width: '100vw'
      }
    };

    // forward wrapper state to scope UI via src URL
    const frameUrl = `/api/app/${this.props.params.orgId}/${location.hash}`;

    return (
      <div>
        <WrapperToolbar organization={this.props.params.orgId} />
        <iframe ref="iframe" src={frameUrl} style={styles.iframe} />
      </div>
    );
  }

  _onFrameStateChanged(hash) {
    // save scope UI state in URL
    //page.show(hash);
  }

}
