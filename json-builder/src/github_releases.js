// this package guards against rate-limitting and has local cache
var ghrequest = require('ghrequest');

var latest;

var POLLING_INTERVAL = 15 * 60 * 1000;

function update() {

  var req = {
    url: '/repos/weaveworks/scope/releases',
    headers: { 'User-Agent': 'My Cool App' }
  };

  ghrequest(req, function(err, res, body) {
    if (err) {
      throw err;
    }
    latest = body[1].tag_name;
  });
}

exports.getLatestScopeRelease = function() {
  return latest;
}

exports.poll = function() {
  update();
  setInterval(update, POLLING_INTERVAL);
}
