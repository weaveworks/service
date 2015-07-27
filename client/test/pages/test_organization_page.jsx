import React from "react/addons";
let { TestUtils } = React.addons;

import OrganizationPage from "../../src/pages/organization/page";


describe("OrganizationPage Component", function() {
  it("should render with data props", function() {
    let data = {
      organization: {
        name: 'Test Organization Name'
      }
    }

    let orgPageComponent = TestUtils.renderIntoDocument(
      <OrganizationPage data={data} />
    );

    let heading = TestUtils.findRenderedDOMComponentWithTag(
      orgPageComponent, "h1");

    expect(heading.getDOMNode().textContent).to.equal('Test Organization Name');
  });
});