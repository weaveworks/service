import React from "react";
import { Styles } from "material-ui";

import { getData } from "../../common/request";
import { Container } from "../../components/container";
import { Column } from "../../components/column";
import Probes from "./probes";
import UserToolbar from "./toolbar";
import Users from "./users";

const ThemeManager = new Styles.ThemeManager();

export default class OrganizationPage extends React.Component {

  static childContextTypes = {
    muiTheme: React.PropTypes.object
  }

  getChildContext() {
    return {
      muiTheme: ThemeManager.getCurrentTheme()
    };
  }

  render() {
    let { name, user, probeToken } = this.props.data.organization;

    return (
      <Container>
        <UserToolbar user={user} />
        <h1><a href="/app/foo">{name}</a></h1>
        <Column>
          <Users org={name} />
        </Column>
        <Column>
          <Probes org={name} probeToken={probeToken} />
        </Column>
      </Container>
    );
  }

  static fetchData = function(params) {
    if (params.orgId) {
      const url = '/api/users/org/' + params.orgId;
      return getData(url);
    }
  }
}
