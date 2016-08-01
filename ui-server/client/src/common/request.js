/**
 * Simple wrapper over XMLHttpRequest that returns a Promise representing
 * the result of making a JSON request.
 *
 * @param  {String} url         the request url
 * @param  {String} method      the request method
 * @param  {Object} requestData the JSON data to send as the request body
 * @return {Promise}            resolves or rejects according to the response
 */
function doRequest(url, method = 'GET', requestData = {}, contentType = null) {
  const request = new XMLHttpRequest();

  if (!request) {
    throw new Error('Could not initialize XMLHttpRequest object!');
  }

  const promise = new Promise((resolve, reject) => {
    request.onreadystatechange = function onReadyStateChange() {
      if (request.readyState === 4) {
        let errorText;
        let responseObject;
        if (request.status >= 200 && request.status < 300) {
          try {
            responseObject = (request.responseText && JSON.parse(request.responseText)) || {};
            responseObject.status = request.status;
          } catch (ex) {
            errorText = `Error while interpreting server message: ${ex}`;
          }
        } else if (request.status === 404) {
          errorText = `Resource ${url} not found`;
        } else if (request.status === 500) {
          errorText = `Server error (${request.responseText})`;
        } else if (request.status >= 501 && request.status < 600) {
          errorText = 'Service is unavailable';
        } else {
          errorText = `Unexpected error: ${request.responseText}`;
        }
        if (responseObject) {
          resolve(responseObject);
        } else {
          reject({errors: [{message: errorText}], status: request.status});
        }
      }
    };

    request.open(method, url);
    if (contentType === 'application/json') {
      request.setRequestHeader('Content-Type', contentType);
      request.send(JSON.stringify(requestData));
    } else {
      request.send();
    }
  });

  return promise;
}

//
// https://leanpub.com/understandinges6/read#leanpub-auto-tagged-templates
// based on function passthru(literals, ...substitutions)
//
// apply encodeURIComponent to all substitutions.
//
export function encodeURIs(...args) {
  const [literals, ...substitutions] = args;
  let result = '';

  // run the loop only for the substitution count
  for (let i = 0; i < substitutions.length; i++) {
    result += literals[i];
    result += encodeURIComponent(substitutions[i]);
  }

  // add the last literal
  result += literals[literals.length - 1];

  return result;
}

export function toQueryString(params) {
  return Object.keys(params).map((k) => `${k}=${encodeURIComponent(params[k])}`).join('&');
}

export function fromQueryString(string) {
  const result = {};
  string.
    slice(1).
    split('&').
    map(pair => pair.split('=', 2)).
    forEach(([k, v]) => { result[k] = decodeURIComponent(v || ''); });
  return result;
}


export function getData(url, params) {
  const getUrl = params ? `${url}?${toQueryString(params)}` : url;
  return doRequest(getUrl, 'GET');
}

export function postData(url, requestData = {}) {
  return doRequest(url, 'POST', requestData, 'application/json');
}

export function putData(url, requestData = {}) {
  return doRequest(url, 'PUT', requestData, 'application/json');
}

export function deleteData(url) {
  return doRequest(url, 'DELETE');
}
