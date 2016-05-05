// this package guards agains rate-limitting and has local cache
var ghrequest = require('ghrequest');

var latest;

function update() {

  var req = {
    url: '/repos/weaveworks/scope/releases',
    headers: { 'User-Agent': 'My Cool App' }
  };

  ghrequest(req, function(err, res, body) {
    if (err) throw err;
    latest = body[1].tag_name;
  });
}

exports.get_latest_scope_release = function() {
  return latest;
}

exports.poll = function() {
  update();
  setInterval(update, 900000) // 15min
}
