/* eslint react/jsx-no-bind:0 */
import React from 'react';
import debug from 'debug';

import { encodeURIs } from '../../common/request';
import PrivatePage from '../../components/private-page';
import { trackView } from '../../common/tracking';

const log = debug('service:prom');

export default class PromWrapperPage extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.handleFrameLoad = this.handleFrameLoad.bind(this);
  }

  componentDidMount() {
    trackView('Prom');
  }

  handleFrameLoad() {
    const css = '' +
      '<style type="text/css">' +
      'body{ padding: 20px 20px 20px 20px; } nav{ display: none; }' +
      '</style>';
    try {
      const iframe = this._iframe.contentDocument;
      iframe.querySelector('head').insertAdjacentHTML('beforeend', css);
    } catch (e) {
      // Security exception
      log('Could not inject CSS into prom frame', e);
    }
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

    const { orgId } = this.props.params;
    const frameUrl = encodeURIs`/api/app/${orgId}/api/prom/graph`;

    return (
      <PrivatePage page="prom" {...this.props.params}>
        <iframe ref={(c) => {this._iframe = c;}} src={frameUrl} style={styles.iframe}
          onLoad={this.handleFrameLoad} />
      </PrivatePage>
    );
  }
}
