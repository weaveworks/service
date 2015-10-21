import React from 'react';
import { Styles, List, ListItem } from 'material-ui';
import { getData } from '../../common/request';
import { Box } from '../../components/box';
import { trackEvent, trackException } from '../../common/tracking';

export default class Probes extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      probes: []
    };
  }

  componentWillMount() {
    this.getProbes();
  }

  getProbes() {
    const url = `/api/org/${this.props.org}/probes`;
    getData(url)
      .then(resp => {
        this.setState({
          probes: resp
        });
        trackEvent('Scope', 'connectedProbes', this.props.org, resp.length);
      }, resp => {
        trackException(resp.errors[0].message);
      });
  }

  renderProbes() {
    if (this.state.probes.length > 0) {
      return this.state.probes.map(probe => {
        return (
          <ListItem primaryText={probe.id} key={probe.id} />
        );
      });
    }

    return (
      <ListItem primaryText="No probes connected" disabled />
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
        <Box>
          <List subheader="Connected probes">
            {probes}
          </List>
        </Box>
        <div style={styles.tokenContainer}>
          <span style={styles.tokenLabel}>Probe token: </span>
          <span style={styles.tokenValue}>{this.props.probeToken}</span>
        </div>
      </div>
    );
  }

}
