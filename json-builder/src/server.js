"use strict";

var _ = require('lodash');

var restify = require('restify');
var Logger = require('bunyan');

var k8s = require('./kubernetes.js');

var log = new Logger({
  name: 'json-builder',
  streams: [{
      stream: process.stdout,
      level: 'debug'
  }],
  serializers: {
    req: Logger.stdSerializers.req,
    res: restify.bunyan.serializers.res,
  }
});

var server = restify.createServer({ name: 'json-builder', log: log });

server.use(restify.queryParser());

server.get('/tag/:tag/weavescope.combined.json', function (req, res, next) {

  var combined = k8s.make_list([
      k8s.make_app_replicationcontroller(req.params),
      k8s.make_app_service(req.params),
      k8s.make_probe_daemonset(req.params)
  ]);

  res.send(combined);
  return next();
});

server.listen(8080, function () {
  log.info({name: server.name, url: server.url}, 'listening');
});

server.pre(function (request, response, next) {
  request.log.info({req: request}, 'started');
  return next();
});

server.on('after', function (req, res, route) {
  req.log.info({res: res}, 'finished');
});
