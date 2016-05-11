"use strict";

var _ = require('lodash');

var restify = require('restify');
var Logger = require('bunyan');

var k8s = require('./kubernetes.js');
var ghr = require('./github_releases.js');
var semver = require('semver');
var yaml = require('js-yaml');

var log = new Logger({
  name: 'launch-generator',
  streams: [{
      stream: process.stdout,
      level: 'debug'
  }],
  serializers: {
    req: Logger.stdSerializers.req,
    res: restify.bunyan.serializers.res,
  }
});

var server = restify.createServer({ name: 'launch-generator', log: log });

server.use(restify.queryParser());

function makeCombinedManifest(params) {
   return k8s.makeList([
       k8s.makeAppReplicationController(params),
       k8s.makeAppService(params),
       k8s.makeProbeDaemonSet(params)
   ]);
}

function makeManifestForService(params) {
   return k8s.makeProbeDaemonSet(params);
}

function validImageTag(params) {

  function current() {
    var ret = semver.valid(ghr.getLatestScopeRelease());
    if (ret === null) {
      log.warn({params: params}, 'this was meant to be unreachable!');
      return 'latest';
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

function makeManifest(params) {

  var _params = { tag: validImageTag(params) };

  if (_.isString(params['service-token'])) {
    if (params['service-token'].length === 0) {
      throw('service token must be set');
    }
    _params.token = params['service-token'];
    return makeManifestForService(_params);
  }

  if (_.isString(params['k8s-service-type'])) {
    var _k8s_service_type = params['k8s-service-type'];
    if (_k8s_service_type === 'NodePort' || _k8s_service_type === 'LoadBalancer') {
      _params.type = _k8s_service_type;
    }
  }

  return makeCombinedManifest(_params);

}

server.get('/launch/k8s/weavescope.json', function (req, res, next) {

  var _manifest = makeManifest(req.params);

  res.send(_manifest);
  return next();
});

server.get('/launch/k8s/weavescope.yaml', function (req, res, next) {

  var _manifest = yaml.safeDump(makeManifest(req.params));

  res.setHeader('content-type', 'application/x-yaml');
  res.send(_manifest);
  return next();
});

ghr.poll();

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
