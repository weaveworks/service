import React from "react";
import { getData } from "../../common/request";

export default class OrganizationPage extends React.Component {
  componentWillMount() {
    console.log("[OrganizationPage] will mount with server response: ", this.props.data.home);
  }

  render() {
    let { name } = this.props.data.organization;

    return (
      <div id="organization-page">
        <h1>{name}</h1>
      </div>
    );
  }

  static fetchData = function(params) {
    return getData("/api/org/foo");
  }
}
