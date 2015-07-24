var express = require('express');
var bodyParser = require('body-parser');
var app = express();

app.use(bodyParser.json()); // for parsing application/json

// Mock data store
var store = {};
store.orgName = 'foo'
store.users = [{
    id: 0,
    email: 'peter@weaver.works',
    lastLogin: null
  }];

/************************************************************
 *
 * Express routes for:
 *   - app.js
 *   - index.html
 *
 *   Sample endpoints to demo async data fetching:
 *     - POST /landing
 *     - POST /home
 *
 ************************************************************/

// Serve application file depending on environment
app.get('/app.js', function(req, res) {
  if (process.env.PRODUCTION) {
    res.sendFile(__dirname + '/build/app.js');
  } else {
    res.redirect('//localhost:9090/build/app.js');
  }
});

// Mock backend

app.get('/api/org/foo', function(req, res) {
  res.json({
    name: orgName
  });
});


app.post('/api/org/foo', function(req, res) {
  orgName = req.body.name;
  res.json({
    name: store.orgName
  });
});

app.get('/api/org/*/probes', function(req, res) {
  res.json([{
    id: 'probe1',
    state: 'connected'
  }]);
});

app.get('/api/org/*/users', function(req, res) {
  res.json(store.users);
});

app.post('/api/org/*/users', function(req, res) {
  store.users.push({
    id: store.users.length,
    email: req.body.email,
    lastLogin: null
  });
  res.json(store.users);
});


// Mock login

app.post('/api/signup', function(req, res) {
  res.json({
    mailSent: !!req.body.email,
    email: req.body.email
  });
});

app.get('/login', function(req, res) {
  res.redirect('org/foo');
});

// Serve index page
app.get('*', function(req, res) {
  res.sendFile(__dirname + '/build/index.html');
});



/*************************************************************
 *
 * Webpack Dev Server
 *
 * See: http://webpack.github.io/docs/webpack-dev-server.html
 *
 *************************************************************/

if (!process.env.PRODUCTION) {
  var webpack = require('webpack');
  var WebpackDevServer = require('webpack-dev-server');
  var config = require('./webpack.local.config');

  new WebpackDevServer(webpack(config), {
    publicPath: config.output.publicPath,
    hot: true,
    noInfo: true,
    historyApiFallback: true
  }).listen(9090, 'localhost', function (err, result) {
    if (err) {
      console.log(err);
    }
  });
}


/******************
 *
 * Express server
 *
 *****************/

var port = process.env.PORT || 8080;
var server = app.listen(port, function () {
  var host = server.address().address;
  var port = server.address().port;

  console.log('Scope Account Service UI listening at http://%s:%s', host, port);
});
