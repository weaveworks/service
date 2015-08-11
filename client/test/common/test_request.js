import { postData } from "../../src/common/request";


describe("Request", function() {
  describe("postData", function() {
    let server;

    beforeEach(function() {
      server = sinon.fakeServer.create();
    });

    afterEach(function() {
      server.restore();
    });


    it("should return promise fulfilled with response", function(done) {
      let promise = postData('/test');
      let responseData = {
        status: 200,
        prop: "Testing",
        didRespond: true
      }

      server.requests[0].respond(
        200,
        { "Content-Type": "application/json" },
        JSON.stringify(responseData)
      );

      promise.then(function(response) {
        expect(response).to.deep.equal(responseData);
        done();
      }).catch(function(response) {
        //
      });
    });

    it("should return promise rejected with error", function(done) {
      let promise = postData('/anotherTest');
      let responseData = {
        error: "Something went wrong!",
        status: 500
      }

      server.requests[0].respond(
        500,
        { "Content-Type": "application/json" },
        JSON.stringify(responseData)
      );

      promise.then(function(response) {

      }).catch(function(response) {
        expect(response).to.deep.equal(responseData);
        done()
      });
    });
  });
});