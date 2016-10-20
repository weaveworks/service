import React from 'react';
import Paper from 'material-ui/Paper';
import RaisedButton from 'material-ui/RaisedButton';
import { grey200 } from 'material-ui/styles/colors';
import { connect } from 'react-redux';
import { browserHistory } from 'react-router';

import { encodeURIs } from '../../common/request';
import { getOrganizationData } from '../../actions';
import Colors from '../../common/colors';
import Box from '../../components/box';
import Column from '../../components/column';
import FlexContainer from '../../components/flex-container';
import PromStatus from './prom-status';

export class PromSetup extends React.Component {

  constructor() {
    super();
    this.handleClickInstance = this.handleClickInstance.bind(this);
  }

  componentDidMount() {
    // get service token
    this.props.getOrganizationData(this.props.params.orgId);
  }

  instanceUrl() {
    return encodeURIs`/prom/${this.props.params.orgId}/wrapper`;
  }

  handleClickInstance() {
    browserHistory.push(this.instanceUrl());
  }

  render() {
    const { instance } = this.props;
    const styles = {
      circle: {
        position: 'absolute',
        left: -56,
        top: -24,
        color: Colors.text4,
        backgroundColor: Colors.background,
        textAlign: 'center',
        lineHeight: 2,
        fontSize: '200%',
      },
      code: {
        marginLeft: -16,
        padding: 16,
        backgroundColor: '#32324B',
        fontFamily: 'monospace',
        color: Colors.text4,
        fontSize: '0.9rem',
        borderRadius: 4
      },
      completed: {
        display: instance && instance.prometheusMetricNames ? 'block' : 'none',
        fontSize: '0.8em',
        borderTop: `2px dotted ${grey200}`,
        padding: 24,
      },
      step: {
        position: 'relative',
        marginTop: '3em',
        marginBottom: '1em'
      },
      steps: {
        marginLeft: -12,
        borderLeft: `1px solid ${Colors.text5}`,
        paddingLeft: 48,
        paddingBottom: 24
      }
    };

    return (
      <FlexContainer style={{ marginTop: 32 }}>
        <Column minWidth="500">
          <h2>Getting started with Prometheus on Weave Cloud</h2>
          <div style={styles.steps}>
            <div style={styles.step}>
              <span style={styles.circle}>1</span>
              <h3>Get Prometheus</h3>
              You can <a href="https://prometheus.io/download/" target="promwebsite">download
              Prometheus from its website</a>. You will need v1.2.1 or later.
            </div>
            <div style={styles.step}>
              <span style={styles.circle}>2</span>
              <h3>Configure Prometheus</h3>
              When you've got Prometheus, you will need to <a href="https://prometheus.io/docs/operating/configuration/"
                target="promwebsite">configure it to discover your services</a> and
              also configure it to send its data to Weave Cloud by adding the
              following top-level stanza to <tt>prometheus.yml</tt>:
            </div>
            <Box>
              <div style={styles.code}>
                <div>remote_write:</div>
                <div>&nbsp;&nbsp;url: https://cloud.weave.works/api/prom/push</div>
                <div>&nbsp;&nbsp;basic_auth:</div>
                <div>&nbsp;&nbsp;&nbsp;&nbsp; {instance && instance.probeToken}</div>
              </div>
            </Box>
            <p>Once Prometheus is sending data to Weave Cloud,
              the connection status on the right side should change to green.</p>
          </div>
        </Column>
        <Column width={300}>
          <Paper style={{marginTop: '4em', marginBottom: '1em', padding: 4}}>
            <PromStatus orgId={this.props.params.orgId} />
            <div style={styles.completed}>
              <p>
                Looks like Prometheus is connected,
                you can take a look at your system:
              </p>
              <div style={{textAlign: 'center'}}>
                <RaisedButton primary
                  label="View Instance" onClick={this.handleClickInstance} />
              </div>
            </div>
          </Paper>
        </Column>
      </FlexContainer>
    );
  }
}

function mapStateToProps(state, ownProps) {
  return {
    instance: state.instances[ownProps.params.orgId]
  };
}

export default connect(mapStateToProps, { getOrganizationData })(PromSetup);
