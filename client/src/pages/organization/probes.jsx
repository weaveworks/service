import React from 'react';
import { Styles } from 'material-ui';
import { getData } from '../../common/request';
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

  getProbes() {
    clearTimeout(this.getProbesTimer);
    const url = `/api/org/${this.props.org}/probes`;
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

  renderProbes() {
    const style = {
      margin: 16
    };
    if (this.state.probes.length > 0) {
      return this.state.probes.map(probe => {
        return (
          <div key={probe.id} style={style}>{probe.id} (connected)</div>
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
