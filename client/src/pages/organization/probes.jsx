import React from 'react';
import _ from 'lodash';
import moment from 'moment';
import { grey400 } from 'material-ui/styles/colors';
import { getData, encodeURIs } from '../../common/request';
import { trackEvent, trackException } from '../../common/tracking';

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

  componentWillUnmount() {
    clearTimeout(this.getProbesTimer);
  }

  getProbes() {
    clearTimeout(this.getProbesTimer);
    const url = encodeURIs`/api/org/${this.props.org}/probes`;
    getData(url)
      .then(resp => {
        this.setState({
          probes: _.sortBy(resp, ['hostname', 'id'])
        });
        trackEvent('Cloud', 'connectedProbes', this.props.org, resp.length);
        this.getProbesTimer = setTimeout(this.getProbes, 5000);
      }, resp => {
        trackException(resp.errors[0].message);
      });
  }

  renderProbes() {
    if (this.state.probes.length > 0) {
      return this.state.probes.map(probe => {
        const probeStyle = {
          margin: 16
        };
        const now = new Date();
        const title = `Last seen: ${moment(probe.lastSeen).from(now)}`;
        return (
          <div key={probe.id} style={probeStyle} title={title} >
            {probe.hostname} (connected)
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
