/* eslint react/jsx-no-bind:0 */
import React from 'react';
import ReactDOM from 'react-dom';
import debug from 'debug';

import { encodeURIs } from '../../common/request';
import PrivatePage from '../../components/private-page';
import { trackView } from '../../common/tracking';

const log = debug('service:prom');

export default class Wrapper extends React.Component {

  constructor() {
    super();

    this.state = {
      activityText: '',
      frameBaseUrl: ''
    };

    this._handleFrameLoad = this._handleFrameLoad.bind(this);
  }

  componentDidMount() {
    trackView('Prom');
  }

  componentDidUpdate() {
    const iframe = ReactDOM.findDOMNode(this._iframe);
    if (iframe) {
      // periodically check iframe's URL and react to changes
      clearInterval(this.frameStateChecker);
      const target = iframe.contentWindow;

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

    const { orgId } = this.props.params;
    const frameUrl = encodeURIs`/api/app/${orgId}/api/prom/graph`;

    return (
      <PrivatePage page="wrapper" {...this.props.params}>
        <iframe ref={(c) => {this._iframe = c;}}
          onLoad={this._handleFrameLoad} src={frameUrl} style={styles.iframe} />}
      </PrivatePage>
    );
  }
}
