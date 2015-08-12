import React from "react";
import { CircularProgress, Styles } from "material-ui";
import { HashLocation } from "react-router";

import { getData } from "../../common/request";
import { Container } from "../../components/container";
import { Column } from "../../components/column";
import Probes from "./probes";
import Toolbar from "../../components/toolbar";
import Users from "./users";

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
      }
    };

    return (
      <div>
        <Toolbar user={this.state.user} organization={this.props.params.orgId} />
        <Container>
          {this.state.name && <div>
            <h1><a href={appUrl}>{this.state.name}</a></h1>
            <Column>
              <Users org={this.state.ame} />
            </Column>
            <Column>
              <Probes org={this.state.name} probeToken={this.state.probeToken} />
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
