import React from "react/addons";
let { TestUtils } = React.addons;

import LandingPage from "../../src/pages/landing/page";


describe("LandingPage Component", function() {
  it("should render", function() {

    let landingPageComponent = TestUtils.renderIntoDocument(
      <LandingPage params={{email: null, token: null}} />
    );

    let heading = TestUtils.findRenderedDOMComponentWithTag(
      landingPageComponent, "h1");

    expect(heading.getDOMNode().textContent).to.equal('Scope as a Service');
  });
});