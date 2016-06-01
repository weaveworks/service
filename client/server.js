var express = require('express');
var bodyParser = require('body-parser');
var proxy = require('http-proxy-middleware');
var url = require('url');

var app = express();
if (process.env.USE_MOCK_BACKEND) {
  app.use(bodyParser.json()); // for parsing application/json
}

var WEBPACK_SERVER_HOST = process.env.WEBPACK_SERVER_HOST || 'localhost';

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
// app.get('/app.js', function(req, res) {
//   if (process.env.NODE_ENV === 'production') {
//     res.sendFile(__dirname + '/build/app.js');
//   } else {
//     res.redirect('//localhost:9090/build/app.js');
//   }
// });

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
} else {

  // Proxy to users
  var usersProxy = proxy({
    target: 'http://localhost:4047',
  });
  app.use('/api/users', usersProxy);

  // Proxy to local Scope
  var backendProxy = proxy({
    ws: true,
    target: 'http://localhost:4042',
    pathRewrite: function(path) {
      // /api/app/icy-snow-65/api/foo -> /api/foo
      return '/' + path.split('/').slice(4).join('/');
    }
  });

  app.use('/api/app', backendProxy);
}

app.get('/landing.jpg', function(req, res) {
  res.sendFile(__dirname + '/src/images/landing.jpg');
});

if (process.env.NODE_ENV === 'production') {
  // serve all precompiled content from build/
  app.use(express.static('build'));
} else {
  // redirect the JS bundles
  app.get(/.*js/, function(req, res) {
    res.redirect('//' + WEBPACK_SERVER_HOST + ':4048' + req.originalUrl);
  });
  // proxy everything else
  var staticProxy = proxy({
    target: 'http://' + WEBPACK_SERVER_HOST + ':4048'
  });
  app.all('*', staticProxy);
}

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
    hot: true,
    noInfo: true,
    historyApiFallback: true,
    stats: { colors: true }
  }).listen(4048, '0.0.0.0', function (err, result) {
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
  if (!process.env.USE_MOCK_BACKEND) {
    console.log('Proxies to local users service on :4047 and to local Scope on :4042');
  }
});

if (!process.env.USE_MOCK_BACKEND) {
  server.on('upgrade', backendProxy.upgrade);
}
