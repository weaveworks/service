/**
 * Simple wrapper over XMLHttpRequest that returns a Promise representing
 * the result of making a JSON request.
 *
 * @param  {String} url         the request url
 * @param  {String} method      the request method
 * @param  {Object} requestData the JSON data to send as the request body
 * @return {Promise}            resolves or rejects according to the response
 */
function doRequest(url, method = 'GET', requestData = {}) {
  const request = new XMLHttpRequest();

  if (!request) {
    throw new Error('Could not initialize XMLHttpRequest object!');
  }

  const promise = new Promise(function requestResolver(resolve, reject) {
    request.onreadystatechange = function onReadyStateChange() {
      if (request.readyState === 4) {
        try {
          const responseObject = JSON.parse(request.responseText);
          responseObject.status = request.status;

          if (request.status === 200) {
            resolve(responseObject);
          } else {
            reject(responseObject);
          }
        } catch (ex) {
          let errorText;
          if (request.status === 404) {
            errorText = `Resource ${url} not found`;
          } else if (request.status === 500) {
            errorText = `Server error (${request.responseText})`;
          } else {
            errorText = 'Unexpected error: ' + ex;
          }
          reject({errors: [{message: errorText}], status: request.status});
        }
      }
    };

    request.open(method, url);
    if (method === 'POST') {
      request.setRequestHeader('Content-Type', 'application/json');
      request.send(JSON.stringify(requestData));
    } else {
      request.send();
    }
  });

  return promise;
}

export function getData(url) {
  return doRequest(url, 'GET');
}

export function postData(url, requestData = {}) {
  return doRequest(url, 'POST', requestData);
}

export function deleteData(url) {
  return doRequest(url, 'DELETE');
}
