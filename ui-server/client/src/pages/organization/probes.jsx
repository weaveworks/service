import React from 'react';
import { cyan500, grey400 } from 'material-ui/styles/colors';

const probeStyle = {
  margin: 16
};

const connectedStyle = {
  position: 'relative',
  fontSize: '90%',
  left: -8,
  top: -1,
  color: cyan500,
  opacity: 0.8
};

export default class Probes extends React.Component {

  renderProbes() {
    if (this.props.probes.length > 0) {
      return this.props.probes.map(probe => {
        const title = `Last seen: ${probe.lastSeen}`;
        return (
          <div key={probe.id} style={probeStyle} title={title} >
            <span style={connectedStyle} className="fa fa-circle" /> {probe.hostname}
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
          <span style={styles.tokenLabel}>Service token: </span>
          <span style={styles.tokenValue}>{this.props.probeToken}</span>
        </div>
      </div>
    );
  }

}
