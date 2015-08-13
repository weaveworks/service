import React from "react";
import { CircularProgress, Styles } from "material-ui";
import { HashLocation } from "react-router";

import Colors from '../../common/colors';
import { getData } from "../../common/request";
import { Box } from "../../components/box";
import { Container } from "../../components/container";
import { Column } from "../../components/column";
import { Logo } from "../../components/logo";
import Probes from "./probes";
import Toolbar from "../../components/toolbar";

const ThemeManager = new Styles.ThemeManager();

export default class OrganizationPage extends React.Component {

  static childContextTypes = {
    muiTheme: React.PropTypes.object
  }

  constructor() {
    super();
    this.state = {
      name: '',
      user: '',
      probeToken: ''
    };

    this._handleOrganizationSuccess = this._handleOrganizationSuccess.bind(this);
    this._handleOrganizationError = this._handleOrganizationError.bind(this);
  }

  getChildContext() {
    return {
      muiTheme: ThemeManager.getCurrentTheme()
    };
  }

  componentDidMount() {
    this._getOrganizationData(this.props.params.orgId);
  }

  render() {
    const appUrl = `#/app/${this.state.name}`;
    const styles = {
      activity: {
        marginTop: 200,
        textAlign: 'center'
      },
      code: {
        padding: 24,
        backgroundColor: '#32324B',
        fontFamilily: 'monospace',
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
      probes: {
        marginTop: 32
      }
    };

    return (
      <div>
        <Toolbar user={this.state.user} organization={this.props.params.orgId} />
        <div style={styles.logoWrapper}>
          <Logo />
        </div>
        <Container>
          {this.state.name && <div style={styles.container}>
            <Column width="50%">
              <h1>Configure your instance</h1>
              <Box>
                <div style={styles.code}>
                  <div>sudo wget -O /usr/local/bin/scope https://git.io/scope-latest</div>
                  <div>sudo chmod a+x /usr/local/bin/scope</div>
                  <div>sudo scope launch --service-token={this.state.probeToken}</div>
                </div>
              </Box>
              <div style={styles.probes}>
                <Probes org={this.state.name} probeToken={this.state.probeToken} />
              </div>
            </Column>
            <Column width="33%">
            </Column>
          </div>}
          {!this.state.name && <div style={styles.activity}>
            <CircularProgress mode="indeterminate" />
          </div>}
        </Container>
      </div>
    );
  }

  _getOrganizationData(organization) {
    if (organization) {
      const url = `/api/users/org/${organization}`;
      getData(url).then(this._handleOrganizationSuccess, this._handleOrganizationError);
    }
  }

  _handleOrganizationSuccess(resp) {
    this.setState(resp);
  }

  _handleOrganizationError(resp) {
    if (resp.status === 401) {
      HashLocation.push('/login');
    } else {
      // TODO show errors
      console.error(resp);
    }
  }
}
