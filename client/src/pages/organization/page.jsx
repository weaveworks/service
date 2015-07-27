import React from "react";
import { getData } from "../../common/request";
import Users from "./users";

export default class OrganizationPage extends React.Component {
  render() {
    let { name, user } = this.props.data.organization;

    return (
      <div id="organization-page">
        <h1>{name}</h1>
        <div>{user}</div>
        <div>
          <h3>Probes</h3>
          <ul>
            <li>Probe1</li>
          </ul>
        </div>
        <Users org={name} />
      </div>
    );
  }

  static fetchData = function(params) {
    if (params.orgId) {
      const url = '/api/org/' + params.orgId;
      return getData(url);
    }
  }
}
