"use strict";

var _ = require('lodash');

var restify = require('restify');
var Logger = require('bunyan');

var k8s = require('./kubernetes.js');
var ghr = require('./github_releases.js');
var semver = require('semver');
var yaml = require('js-yaml');

ghr.poll();

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

function make_combined_manifest(params) {
   return k8s.make_list([
       k8s.make_app_replicationcontroller(params),
       k8s.make_app_service(params),
       k8s.make_probe_daemonset(params)
   ]);
}

function make_manifest_for_service(params) {
   return k8s.make_probe_daemonset(params);
}

function valid_image_tag(params) {

  function current() {
    var ret = semver.valid(ghr.get_latest_scope_release());
    if (ret === null) {
      return 'latest'; // should be unreachable, but just in case
    }
    return ret;
  }

  if (params.v !== undefined && typeof params.v === 'string') {
    var v = semver.valid(params.v);
    if (v !== null) {
      return v;
    } else if (v === null && params.v.length === 0) {
      return current(); // when param is empty `?v=&foo=bar`
    } else {
      return params.v; // may be an arbitrary custom image tag
    }
  } else {
    return current();
  }
}

function make_manifest(params) {

  var _params = { tag: valid_image_tag(params) };

  if (params['service-token'] !== undefined && typeof params['service-token'] === 'string') {
    if (params['service-token'].length === 0) {
      throw('service token must be set');
    }
    _params.token = params['service-token'];
    return make_manifest_for_service(_params);
  }

  if (params['k8s-service-type'] !== undefined && typeof params['k8s-service-type'] === 'string') {
    var _k8s_service_type = params['k8s-service-type'];
    if (_k8s_service_type === 'NodePort' || _k8s_service_type === 'LoadBalancer') {
      _params.type = _k8s_service_type;
    }
  }

  return make_combined_manifest(_params);

}

server.get('/k8s-gen/weavescope.json', function (req, res, next) {

  var _manifest = make_manifest(req.params);

  res.send(_manifest);
  return next();
});

server.get('/k8s-gen/weavescope.yaml', function (req, res, next) {

  var _manifest = yaml.safeDump(make_manifest(req.params));

  res.setHeader('content-type', 'application/x-yaml');
  res.send(_manifest);
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
