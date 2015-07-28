/**
 * Simple wrapper over XMLHttpRequest that returns a Promise representing
 * the result of making a GET request.
 *
 * @param  {String} url         the request url
 * @return {Promise}            resolves or rejects according to the response
 */
export let getData = function(url) {
  let request = new XMLHttpRequest();

  if (!request) {
    throw new Error('Could not initialize XMLHttpRequest object!');
  }

  let promise = new Promise(function(resolve, reject) {
    request.onreadystatechange = function() {
      if (request.readyState === 4) {
        let responseObject = JSON.parse(request.responseText);

        if (request.status === 200) {
          resolve(responseObject);
        } else {
          reject(responseObject);
        }
      }
    }

    request.open('GET', url);
    request.send();
  });

  return promise;
};

/**
 * Simple wrapper over XMLHttpRequest that returns a Promise representing
 * the result of making a JSON POST request.
 *
 * @param  {String} url         the request url
 * @param  {Object} requestData the JSON data to send as the request body
 * @return {Promise}            resolves or rejects according to the response
 */
export let postData = function(url, requestData = {}) {
  let request = new XMLHttpRequest();

  if (!request) {
    throw new Error('Could not initialize XMLHttpRequest object!');
  }

  let promise = new Promise(function(resolve, reject) {
    request.onreadystatechange = function() {
      if (request.readyState === 4) {
        let responseObject = JSON.parse(request.responseText);

        if (request.status === 200) {
          resolve(responseObject);
        } else {
          reject(responseObject);
        }
      }
    }

    request.open('POST', url);
    request.setRequestHeader('Content-Type', 'application/json');
    request.send(JSON.stringify(requestData));
  });

  return promise;
};


/**
 * Simple wrapper over XMLHttpRequest that returns a Promise representing
 * the result of making a JSON POST request.
 *
 * @param  {String} url         the request url
 * @param  {Object} requestData the JSON data to send as the request body
 * @return {Promise}            resolves or rejects according to the response
 */
export let deleteData = function(url) {
  let request = new XMLHttpRequest();

  if (!request) {
    throw new Error('Could not initialize XMLHttpRequest object!');
  }

  let promise = new Promise(function(resolve, reject) {
    request.onreadystatechange = function() {
      if (request.readyState === 4) {
        let responseObject = JSON.parse(request.responseText);

        if (request.status === 200) {
          resolve(responseObject);
        } else {
          reject(responseObject);
        }
      }
    }

    request.open('DELETE', url);
    request.send();
  });

  return promise;
};