import React from 'react';
import CircularProgress from 'material-ui/CircularProgress';
import Paper from 'material-ui/Paper';
import RaisedButton from 'material-ui/RaisedButton';
import Snackbar from 'material-ui/Snackbar';
import { hashHistory } from 'react-router';
import sortBy from 'lodash/sortBy';
import { grey200 } from 'material-ui/styles/colors';

import Colors from '../../common/colors';
import { getProbes } from '../../common/api';
import { getData, encodeURIs } from '../../common/request';
import { Box } from '../../components/box';
import { FlexContainer } from '../../components/flex-container';
import { Column } from '../../components/column';
import { Logo } from '../../components/logo';
import Probes from './probes';
import Users from './users';
import Toolbar from '../../components/toolbar';
import { trackEvent, trackException, trackView } from '../../common/tracking';

export default class OrganizationPage extends React.Component {

  constructor() {
    super();
    this.state = {
      name: '',
      user: '',
      probeToken: '',
      probes: [],
      showHelp: false,
      errors: null
    };

    this.handleClickInstance = this.handleClickInstance.bind(this);
    this._handleOrganizationSuccess = this._handleOrganizationSuccess.bind(this);
    this._handleOrganizationError = this._handleOrganizationError.bind(this);
    this.loadProbesTimer = 0;
    this.loadProbes = this.loadProbes.bind(this);
    this.expandHelp = this.expandHelp.bind(this);
    this.setErrors = this.setErrors.bind(this);
    this.clearErrors = this.clearErrors.bind(this);
  }

  componentDidMount() {
    this._getOrganizationData(this.props.params.orgId);
    this.loadProbes();
    trackView('Organization');
  }

  componentWillUnmount() {
    clearTimeout(this.loadProbesTimer);
  }

  clearErrors() {
    this.setErrors(null);
  }

  setErrors(errors = null) {
    this.setState(Object.assign({}, this.state, {errors}));
  }

  expandHelp(ev) {
    ev.preventDefault();
    if (!this.state.showHelp) {
      this.setState({ showHelp: true });
    }
  }

  instanceUrl() {
    return encodeURIs`/app/${this.props.params.orgId}`;
  }

  _getOrganizationData(organization) {
    if (organization) {
      const url = encodeURIs`/api/users/org/${organization}`;
      getData(url).then(this._handleOrganizationSuccess, this._handleOrganizationError);
    }
  }

  loadProbes() {
    clearTimeout(this.loadProbesTimer);
    const org = this.props.params.orgId;
    getProbes(org)
      .then(resp => {
        this.setState({
          probes: sortBy(resp, ['hostname', 'id'])
        });
        trackEvent('Cloud', 'connectedProbes', org, resp.length);
        this.loadProbesTimer = setTimeout(this.loadProbes, 5000);
      }, resp => {
        trackException(resp.errors[0].message);
        this.setErrors([{message: 'Failed to load probes'}]);
      });
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
    const hasProbes = this.state.probes && this.state.probes.length > 0;
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
      completed: {
        borderTop: `2px dotted ${grey200}`,
        display: hasProbes ? 'block' : 'none',
        marginTop: 24,
        marginBottom: 24
      },
      container: {
        marginTop: 96
      },
      help: {
        borderTop: `2px dotted ${grey200}`,
        display: hasProbes ? 'none' : 'block',
        marginTop: 24,
        marginBottom: 24,
        lineHeight: 1.5,
        fontSize: '85%'
      },
      helpHint: {
        fontSize: '95%',
        textAlign: 'center'
      },
      helpBlock: {
        display: this.state.showHelp ? 'block' : 'none',
        marginLeft: '-1em'
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
        top: -24,
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
        paddingLeft: 48,
        paddingBottom: 24
      }
    };

    return (
      <div style={{height: '100%', position: 'relative'}}>
        <Snackbar
          action="ok"
          open={Boolean(this.state.errors)}
          message={this.state.errors ? this.state.errors.map(e => e.message).join('. ') : ''}
          onActionTouchTap={this.clearErrors}
          onRequestClose={this.clearErrors}
        />
        <Toolbar user={this.state.user} organization={this.props.params.orgId} />
        <div style={styles.logoWrapper}>
          <Logo />
        </div>
        {this.state.name && <div style={styles.container}>
          <FlexContainer>
            <Column minWidth="500">
              <h2>Configure <nobr>{this.props.params.orgId}</nobr></h2>
              <div style={styles.steps}>
                <div style={styles.step}>
                  <span style={styles.circle}>1</span>
                  <h3>Launch the Weave Cloud Probe</h3>
                  Run the following commands on your Docker hosts to connect them
                  as probes to this Weave Cloud instance:
                </div>
                <Box>
                  <div style={styles.code}>
                    <div>sudo curl -L git.io/scope -o /usr/local/bin/scope</div>
                    <div>sudo chmod a+x /usr/local/bin/scope</div>
                    <div>scope launch --service-token={this.state.probeToken}</div>
                  </div>
                </Box>
                <div style={styles.step}>
                  <span style={styles.circle}>2</span>
                  <h3>Try our Reference Application</h3>
                  If you don't have an application of your own, try our <a href="https://github.com/weaveworks/weaveDemo">Reference Application</a> using <a href="https://docs.docker.com/compose/install/">Docker Compose</a>:
                </div>
                <Box>
                  <div style={styles.code}>
                    <div>curl -L git.io/weavedemo-compose.yml -o docker-compose.yml</div>
                    <div>docker-compose up</div>
                  </div>
                </Box>
                <div style={styles.step}>
                  <span style={styles.circle}>3</span>
                  <h3>Invite team members</h3>
                  <p>
                    Send invites to allow other members of your team to view this
                    Weave Cloud instance.
                    You can also come back and do this later.
                  </p>
                  <Users org={this.state.name} />
                </div>
              </div>
            </Column>
            <Column width="400">
              <Paper style={{marginTop: '4em', marginBottom: '1em'}}>
                <div style={styles.probes}>
                  <h3>Probes</h3>
                  <Probes probes={this.state.probes} probeToken={this.state.probeToken} />
                  <div style={styles.completed}>
                    <p>
                      Looks like probes are connected,
                      you can take a look at your system:
                    </p>
                    <div style={{textAlign: 'center'}}>
                      {/* TODO this should be made primary only when probes are connected */}
                      <RaisedButton primary
                        label="View Instance" onClick={this.handleClickInstance} />
                    </div>
                  </div>
                  <div style={styles.help}>
                    <p style={styles.helpHint}>
                      Have you started a probe and don't see it in this list?
                      {!this.state.showHelp && <span>
                        <br /><a href="#" onClick={this.expandHelp}>Show Help</a>
                      </span>}
                    </p>
                    <ol style={styles.helpBlock}>
                      <li>Make sure that the token passed
                        to <code>scope launch</code> is correct.</li>
                      <li>
                        Check the scope probe logs for errors by running<br />
                          <code>docker logs weavescope</code>
                        <ul>
                          <li>If you see 401 errors, check again that the token is correct</li>
                          <li>If you see any other errors, please <a
                            href="https://www.weave.works/help/" target="support">contact support</a></li>
                        </ul>
                      </li>
                    </ol>
                  </div>
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
