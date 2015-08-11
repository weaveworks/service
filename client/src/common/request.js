/**
 * Simple wrapper over XMLHttpRequest that returns a Promise representing
 * the result of making a JSON request.
 *
 * @param  {String} url         the request url
 * @param  {String} method      the request method
 * @param  {Object} requestData the JSON data to send as the request body
 * @return {Promise}            resolves or rejects according to the response
 */
let doRequest = function(url, method = 'GET', requestData = {}) {
  let request = new XMLHttpRequest();

  if (!request) {
    throw new Error('Could not initialize XMLHttpRequest object!');
  }

  let promise = new Promise(function(resolve, reject) {
    request.onreadystatechange = function() {
      if (request.readyState === 4) {
        try {
          let responseObject = JSON.parse(request.responseText);
          responseObject.status = request.status;

          if (request.status === 200) {
            resolve(responseObject);
          } else {
            reject(responseObject);
          }
        } catch (e) {
          let errorText;
          if (request.status === 404) {
            errorText = `Resource ${url} not found`;
          } else if (request.status === 500) {
            errorText = `Server error (${request.responseText})`;
          } else {
            errorText = 'Unexpected error: ' + e;
          }
          reject({errors: [{message: errorText}], status: request.status});
        }
      }
    }

    request.open(method, url);
    if (method === 'POST') {
      request.setRequestHeader('Content-Type', 'application/json');
      request.send(JSON.stringify(requestData));
    } else {
      request.send();
    }
  });

  return promise;
};

export let getData = function(url) {
  return doRequest(url, 'GET');
};

export let postData = function(url, requestData = {}) {
  return doRequest(url, 'POST', requestData);
};

export let deleteData = function(url) {
  return doRequest(url, 'DELETE');
};