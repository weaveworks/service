import React from 'react';
import { CircularProgress, Paper } from 'material-ui';
import ThemeManager from 'material-ui/lib/styles/theme-manager';
import LightRawTheme from 'material-ui/lib/styles/raw-themes/light-raw-theme';

import Colors from '../../common/colors';
import { getData, encodeURIs } from '../../common/request';
import { Box } from '../../components/box';
import { FlexContainer } from '../../components/flex-container';
import { Column } from '../../components/column';
import { Logo } from '../../components/logo';
import Probes from './probes';
import Toolbar from '../../components/toolbar';
import { trackException, trackView } from '../../common/tracking';

export default class OrganizationPage extends React.Component {

  constructor() {
    super();
    this.state = {
      muiTheme: ThemeManager.getMuiTheme(LightRawTheme),
      name: '',
      user: '',
      probeToken: ''
    };

    this._handleOrganizationSuccess = this._handleOrganizationSuccess.bind(this);
    this._handleOrganizationError = this._handleOrganizationError.bind(this);
  }

  getChildContext() {
    return {
      muiTheme: this.state.muiTheme
    };
  }

  componentDidMount() {
    this._getOrganizationData(this.props.params.orgId);
    trackView('Organization');
  }

  _getOrganizationData(organization) {
    if (organization) {
      const url = encodeURIs`/api/users/org/${organization}`;
      getData(url).then(this._handleOrganizationSuccess, this._handleOrganizationError);
    }
  }

  _handleOrganizationSuccess(resp) {
    this.setState(resp);
  }

  _handleOrganizationError(resp) {
    if (resp.status === 401) {
      this.props.history.push('/login');
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
        padding: 24,
        backgroundColor: '#32324B',
        fontFamily: 'monospace',
        color: Colors.text2
      },
      container: {
        marginTop: 128
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
        left: '-3.5em',
        top: '-0.75em',
        width: '2.5em',
        height: '2.5em',
        borderRadius: '50%',
        backgroundColor: Colors.text3,
        color: 'white',
        textAlign: 'center',
        lineHeight: 2.5,
        fontSize: '125%',
        boxShadow: 'rgba(0, 0, 0, 0.117647) 0px 1px 6px, rgba(0, 0, 0, 0.239216) 0px 1px 4px'
      },
      probes: {
        padding: '24'
      },
      step: {
        position: 'relative',
        marginTop: '3em',
        marginBottom: '1em'
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
            <Column minWidth="400">
              <h1>Configure your app</h1>
              <div style={styles.step}>
                <span style={styles.circle}>1</span>
                Run the following commands on your Docker hosts to connect them as probes to this Weave Scope instance:
              </div>
              <Box>
                <div style={styles.code}>
                  <div>sudo wget -O /usr/local/bin/scope \<br />&nbsp;&nbsp;https://github.com/weaveworks/scope/releases/download/v0.13.0/scope</div>
                  <div>sudo chmod a+x /usr/local/bin/scope</div>
                  <div>sudo scope launch --service-token={this.state.probeToken}</div>
                </div>
              </Box>
              <div style={styles.step}>
                <span style={styles.circle}>2</span>
                Once you have started <code>scope</code> on your Docker hosts, click "My Scope" in the top right.
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

OrganizationPage.childContextTypes = {
  muiTheme: React.PropTypes.object
};
