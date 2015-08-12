import React from "react/addons";
import { RouteHandler } from "react-router";
let { TestUtils } = React.addons;

import stubRouterContext from "./stub_router_context";
import Router from "../../src/router";


describe("Router", function() {
  let routerComponent;

  beforeEach(function() {
    let StubbedRouter = stubRouterContext(Router);
    routerComponent = TestUtils.renderIntoDocument(<StubbedRouter />);
  });

  it("should return routes", function() {
    let routes = Router.getRoutes();

    expect(routes).to.exist;
  });

  it("should include <RouterHandler> component", function() {
    let handler = TestUtils.findRenderedComponentWithType(
      routerComponent, RouteHandler);

      expect(handler).to.exist;
  });
});
