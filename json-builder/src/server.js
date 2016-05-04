"use strict";

var _ = require('lodash');

var restify = require('restify');
var Logger = require('bunyan');

var k8s = require('./kubernetes.js');
var ghr = require('./github_releases.js');

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

function handle_combined_manifest(params) {
   return k8s.make_list([
       k8s.make_app_replicationcontroller(params),
       k8s.make_app_service(params),
       k8s.make_probe_daemonset(params)
   ]);
}
function handle_manifest_for_service(params) {
   return k8s.make_probe_daemonset(params);
}

server.get('/k8s/scope/:tag/combined.json', function (req, res, next) {
  res.send(handle_combined_manifest({tag: req.params.tag}));
  return next();
});

server.get('/k8s/scope/combined.json', function (req, res, next) {
  ghr.get_latest_scope_release(log, function (tag) {
    res.send(handle_combined_manifest({tag: tag}));
    return next();
  });
});
server.get('/k8s/scope/:tag/token/:token/combined.json', function (req, res, next) {
  res.send(handle_manifest_for_service({tag: req.params.tag, token: req.params.token}));
  return next();
});

server.get('/k8s/scope/token/:token/combined.json', function (req, res, next) {
  ghr.get_latest_scope_release(log, function (tag) {
    res.send(handle_manifest_for_service({tag: tag, token: req.params.token}));
    return next();
  });
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
