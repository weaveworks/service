var express = require('express');
var bodyParser = require('body-parser');
var proxy = require('proxy-middleware');
var url = require('url');

var app = express();
if (process.env.USE_MOCK_BACKEND) {
  app.use(bodyParser.json()); // for parsing application/json
}

// http proxy
var httpProxy = require('http-proxy');
var proxy = httpProxy.createProxyServer({ ws: true });


// Mock data store
var store = {};
store.orgName = 'foo'
store.users = [{
    id: 0,
    email: 'peter@weave.works',
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
  if (process.env.NODE_ENV === 'production') {
    res.sendFile(__dirname + '/build/app.js');
  } else {
    res.redirect('//localhost:9090/build/app.js');
  }
});

// Proxy to backend

app.use('/api', proxy(url.parse('http://localhost:4047/api')));

// Mock backend

if (process.env.USE_MOCK_BACKEND) {
  app.get('/api/org/foo', function(req, res) {
    res.json({
      user: store.users[0].email,
      name: store.orgName
    });
  });


  app.post('/api/org/foo', function(req, res) {
    store.orgName = req.body.name;
    res.json({
      user: store.users[0].email,
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

  app.delete('/api/org/*/users/:userId', function(req, res) {
    var id = parseInt(req.params.userId);
    for(var i = store.users.length - 1; i >= 0; i--) {
      if(store.users[i].id === id) {
        store.users.splice(i, 1);
      }
    }
    res.json(store.users);
  });

  app.post('/api/signup', function(req, res) {
    res.json({
      mailSent: !!req.body.email,
      email: req.body.email
    });
  });  

  app.get('/login', function(req, res) {
    res.redirect('org/foo');
  });
}

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

if (process.env.NODE_ENV !== 'production') {
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

var port = process.env.PORT || 4046;
var server = app.listen(port, function () {
  var host = server.address().address;
  var port = server.address().port;

  console.log('Scope Account Service UI listening at http://%s:%s', host, port);
});
