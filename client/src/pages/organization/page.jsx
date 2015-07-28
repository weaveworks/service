import React from "react";
import { getData } from "../../common/request";
import { Container } from "../../components/container";
import { Column } from "../../components/column";
import Probes from "./probes";
import UserToolbar from "./toolbar";
import Users from "./users";

export default class OrganizationPage extends React.Component {

  render() {
    let { name, user } = this.props.data.organization;

    return (
      <Container>
        <UserToolbar user={user} />
        <h1>{name}</h1>
        <Column>
          <Users org={name} />
        </Column>
        <Column>
          <Probes org={name} />
        </Column>
      </Container>
    );
  }

  static fetchData = function(params) {
    if (params.orgId) {
      const url = '/api/org/' + params.orgId;
      return getData(url);
    }
  }
}
