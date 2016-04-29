// this package guards agains rate-limitting and has local cache
var ghrequest = require('ghrequest');

var req = {
  url: '/repos/weaveworks/scope/releases',
  headers: { 'User-Agent': 'My Cool App' }
};

exports.get_latest_scope_release = function(log, callback) {
  ghrequest(req, function(err, res, body) {
    if (err) throw err;
    callback(body[1].tag_name);
  });
};
