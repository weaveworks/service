import React from 'react';
import CircularProgress from 'material-ui/CircularProgress';
import Paper from 'material-ui/Paper';
import RaisedButton from 'material-ui/RaisedButton';
import { hashHistory } from 'react-router';

import Colors from '../../common/colors';
import { getData, encodeURIs } from '../../common/request';
import { Box } from '../../components/box';
import { FlexContainer } from '../../components/flex-container';
import { Column } from '../../components/column';
import { Logo } from '../../components/logo';
import Probes from './probes';
import Users from './users';
import Toolbar from '../../components/toolbar';
import { trackException, trackView } from '../../common/tracking';

export default class OrganizationPage extends React.Component {

  constructor() {
    super();
    this.state = {
      name: '',
      user: '',
      probeToken: ''
    };

    this.handleClickInstance = this.handleClickInstance.bind(this);
    this._handleOrganizationSuccess = this._handleOrganizationSuccess.bind(this);
    this._handleOrganizationError = this._handleOrganizationError.bind(this);
  }

  componentDidMount() {
    this._getOrganizationData(this.props.params.orgId);
    trackView('Organization');
  }

  instanceUrl() {
    return encodeURIs`#/app/${this.props.params.orgId}`;
  }

  _getOrganizationData(organization) {
    if (organization) {
      const url = encodeURIs`/api/users/org/${organization}`;
      getData(url).then(this._handleOrganizationSuccess, this._handleOrganizationError);
    }
  }

  handleClickInstance() {
    hashHistory.push(this.instanceUrl());
  }

  _handleOrganizationSuccess(resp) {
    this.setState(resp);
  }

  _handleOrganizationError(resp) {
    if (resp.status === 401) {
      hashHistory.push('/login');
    } else {
      // TODO show errors
      trackException(resp);
    }
  }

  render() {
    const styles = {
      activity: {
        marginTop: 200,
        textAlign: 'center'
      },
      clear: {
        clear: 'both'
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
      container: {
        marginTop: 96
      },
      logoWrapper: {
        position: 'absolute',
        width: 250,
        height: 64,
        left: 64,
        top: 32 + 51 - 3
      },
      circle: {
        position: 'absolute',
        left: -56,
        top: -12,
        color: Colors.text4,
        backgroundColor: Colors.background,
        textAlign: 'center',
        lineHeight: 2,
        fontSize: '200%',
      },
      probes: {
        padding: 24
      },
      step: {
        position: 'relative',
        marginTop: '3em',
        marginBottom: '1em'
      },
      steps: {
        marginLeft: -12,
        borderLeft: `1px solid ${Colors.text5}`,
        paddingLeft: 48
      }
    };

    return (
      <div style={{height: '100%', overflowY: 'scroll', position: 'relative'}}>
        <Toolbar user={this.state.user} organization={this.props.params.orgId} />
        <div style={styles.logoWrapper}>
          <Logo />
        </div>
        {this.state.name && <div style={styles.container}>
          <FlexContainer>
            <Column minWidth="500">
              <div style={styles.steps}>
                <div style={styles.step}>
                  <span style={styles.circle}>1</span>
                  <h2>Configure your app</h2>
                  Run the following commands on your Docker hosts to connect them
                  as probes to this Weave Cloud instance:
                </div>
                <Box>
                  <div style={styles.code}>
                    <div>sudo wget -O /usr/local/bin/scope \<br />&nbsp;&nbsp;https://github.com/weaveworks/scope/releases/download/v0.15.0/scope</div>
                    <div>sudo chmod a+x /usr/local/bin/scope</div>
                    <div>sudo scope launch --service-token={this.state.probeToken}</div>
                  </div>
                </Box>
                <div style={styles.step}>
                  <span style={styles.circle}>2</span>
                  <h2>Invite members</h2>
                  <Users org={this.state.name} />
                </div>
                <div style={styles.step}>
                  <span style={styles.circle}>3</span>
                  <h2>View Instance</h2>
                  <p>
                    Once you have started the probe on your Docker hosts,
                    you can take a look at your system:
                  </p>
                  <div style={{textAlign: 'center'}}>
                    {/* TODO this should be made primary only when probes are connected */}
                    <RaisedButton primary
                      label="View Instance" onClick={this.handleClickInstance} />
                  </div>
                </div>
              </div>
            </Column>
            <Column width="400">
              <Paper style={{marginTop: '4em', marginBottom: '1em'}}>
                <div style={styles.probes}>
                  <h3>Probes</h3>
                  <Probes org={this.state.name} probeToken={this.state.probeToken} />
                </div>
              </Paper>
            </Column>
          </FlexContainer>
          {!this.state.name && <div style={styles.activity}>
            <CircularProgress mode="indeterminate" />
          </div>}
        </div>}
      </div>
    );
  }

}
