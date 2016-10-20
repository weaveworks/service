/* eslint react/jsx-no-bind:0 */
import React from 'react';
import debug from 'debug';
import { connect } from 'react-redux';
import Paper from 'material-ui/Paper';

import { encodeURIs } from '../../common/request';
import { focusFrame } from '../../actions';
import PromBar from './prom-bar';

const log = debug('service:prom');

class PromWrapper extends React.Component {

  constructor(props, context) {
    super(props, context);

    this.handleFrameLoad = this.handleFrameLoad.bind(this);
    this.handleFrameFocus = this.handleFrameFocus.bind(this);
    this.clickFrameExecuteButton = this.clickFrameExecuteButton.bind(this);
    this.setExpressionField = this.setExpressionField.bind(this);
  }

  handleFrameLoad() {
    const css = '' +
      '<style type="text/css">' +
      'body{ padding: 8px 16px 16px; background-color: #f7f7f9 } nav{ display: none; }' +
      '</style>';
    try {
      const iframe = this._iframe.contentDocument;
      iframe.querySelector('head').insertAdjacentHTML('beforeend', css);
    } catch (e) {
      // Security exception
      log('Could not inject CSS into prom frame', e);
    }

    this._iframe.contentWindow.addEventListener('focus', this.handleFrameFocus, true);
  }

  handleFrameFocus() {
    this.props.focusFrame();
  }

  setExpressionField(text) {
    const iframe = this._iframe.contentDocument;
    const field = iframe.querySelector('textarea');
    if (field) {
      field.value = text;
    }
  }

  clickFrameExecuteButton() {
    const iframe = this._iframe.contentDocument;
    const button = iframe.querySelector('.execute_btn');
    if (button) {
      button.click();
    }
  }

  render() {
    const styles = {
      paper: {
        backgroundColor: 'transparent',
        marginBottom: 4
      },
      iframe: {
        display: 'block',
        border: 'none',
        height: 'calc(100vh - 164px)',
        width: '100%'
      }
    };

    const { orgId } = this.props.params;
    const frameUrl = encodeURIs`/api/app/${orgId}/api/prom/graph`;

    return (
      <div>
        <Paper zDepth={1} style={styles.paper}>
          <PromBar orgId={orgId}
            clickFrameExecuteButton={this.clickFrameExecuteButton}
            setExpressionField={this.setExpressionField}
            />
        </Paper>
        <iframe
          ref={(c) => {this._iframe = c;}}
          src={frameUrl} style={styles.iframe}
          onLoad={this.handleFrameLoad}
        />
      </div>
    );
  }
}

export default connect(null, { focusFrame })(PromWrapper);
