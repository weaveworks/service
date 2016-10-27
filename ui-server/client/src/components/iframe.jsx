/* eslint react/jsx-no-bind:0 */
import React from 'react';
import { connect } from 'react-redux';

import { focusFrame } from '../actions';

class IFrame extends React.Component {

  constructor() {
    super();

    this.handleFrameFocus = this.handleFrameFocus.bind(this);
    this.handleFrameLoad = this.handleFrameLoad.bind(this);
  }

  handleFrameLoad() {
    this._iframe.contentWindow.addEventListener('focus', this.handleFrameFocus, true);
  }

  handleFrameFocus() {
    this.props.dispatch(focusFrame);
  }

  render() {
    const styles = {
      iframe: {
        display: 'block',
        border: 'none',
        height: 'calc(100vh - 56px)',
        width: '100%'
      }
    };

    return (
      <iframe ref={(c) => {this._iframe = c;}} onLoad={this.handleFrameLoad}
        src={this.props.src} style={styles.iframe} />
    );
  }
}

export default connect()(IFrame);
