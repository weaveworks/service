import React from 'react';
import { Styles } from 'material-ui';
import { getData, encodeURIs } from '../../common/request';
import { trackEvent, trackException } from '../../common/tracking';

// momentjs doesn't support ES6 importing at the moment... :).
const moment = require('moment');

const STILL_CONNECTED_TIME_DELTA = 15000;

function getTimeDiff(d1, d2) {
  return (d2.getTime() - d1.getTime());
}

export default class Probes extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      probes: []
    };
    this.getProbesTimer = 0;
    this.getProbes = this.getProbes.bind(this);
  }

  componentDidMount() {
    this.getProbes();
  }

  getProbes() {
    clearTimeout(this.getProbesTimer);
    const url = encodeURIs`/api/org/${this.props.org}/probes`;
    getData(url)
      .then(resp => {
        this.setState({
          probes: resp
        });
        trackEvent('Scope', 'connectedProbes', this.props.org, resp.length);
        this.getProbesTimer = setTimeout(this.getProbes, 5000);
      }, resp => {
        trackException(resp.errors[0].message);
      });
  }

  isProbeConnected(probe) {
    return (
      getTimeDiff(new Date(probe.lastSeen), new Date()) <
      STILL_CONNECTED_TIME_DELTA
    );
  }

  renderProbes() {
    if (this.state.probes.length > 0) {
      return this.state.probes.map(probe => {
        const isConnected = this.isProbeConnected(probe);
        const probeStyle = {
          margin: 16,
          opacity: isConnected ? 1 : 0.5
        };
        const title = `Last seen: ${moment(probe.lastSeen).fromNow()}`;
        return (
          <div key={probe.id} style={probeStyle} title={title} >
            {probe.id} {isConnected ? '(connected)' : '(disconnected)'}
          </div>
        );
      });
    }

    return (
      <div>No probes connected</div>
    );
  }

  render() {
    const probes = this.renderProbes();

    const styles = {
      tokenContainer: {
        marginTop: '2em',
        textAlign: 'center',
        fontSize: '80%',
        color: Styles.Colors.grey400
      },
      tokenValue: {
        fontFamily: 'monospace',
        fontSize: '130%'
      }
    };

    return (
      <div>
        <div>
          {probes}
        </div>
        <div style={styles.tokenContainer}>
          <span style={styles.tokenLabel}>Probe token: </span>
          <span style={styles.tokenValue}>{this.props.probeToken}</span>
        </div>
      </div>
    );
  }

}
