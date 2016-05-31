import React from 'react';
import _ from 'lodash';
import moment from 'moment';
import { grey400 } from 'material-ui/styles/colors';
import { getData, encodeURIs } from '../../common/request';
import { trackEvent, trackException } from '../../common/tracking';

const STILL_CONNECTED_TIME_DELTA = 15000;

function getTimeDiff(d1, d2) {
  return (d2.getTime() - d1.getTime());
}

function isProbeConnected(probe) {
  return (
    getTimeDiff(new Date(probe.lastSeen), new Date()) <
    STILL_CONNECTED_TIME_DELTA
  );
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
          // negate isProbeConnected. In JS: false < true
          probes: _.sortByAll(resp, [_.negate(isProbeConnected), 'id'])
        });
        trackEvent('Scope', 'connectedProbes', this.props.org, resp.length);
        this.getProbesTimer = setTimeout(this.getProbes, 5000);
      }, resp => {
        trackException(resp.errors[0].message);
      });
  }

  renderProbes() {
    if (this.state.probes.length > 0) {
      return this.state.probes.map(probe => {
        const isConnected = isProbeConnected(probe);
        const probeStyle = {
          margin: 16,
          opacity: isConnected ? 1 : 0.5
        };
        const now = new Date();
        const title = `Last seen: ${moment(probe.lastSeen).from(now)}`;
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
        color: grey400
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
