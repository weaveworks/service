"use strict";

var _ = require('lodash');

var mergeSpec = {
  ReplicationController: function(_metadata, _spec, _params) {
    return {
      template: _.merge({}, _.clone(_metadata), _spec)
    };
  },
  Service: function(_metadata, _spec, _params) {
    return _.merge({}, _params, {
      selector: _.clone(_metadata).metadata.labels
    });
  },
  DaemonSet: function(_metadata, _spec, _params) {
    return {
      template: _.merge({}, _.clone(_metadata), _spec)
    };
  },
};

function makeResource(apiVersion, kind, component, _spec, _params) {

  var _name = [ 'weavescope' , component ].join('-');
  var _metadata = {
    metadata: {
      labels: { 'app': 'weavescope', 'weavescope-component': _name }
    }
  };

  return _.merge({}, _.clone(_metadata), {
    apiVersion: apiVersion,
    kind: kind,
    metadata: { name: _name },
    spec: _.merge({}, _params, mergeSpec[kind](_metadata, _spec, _params))
  });
}


exports.makeAppReplicationController = function appReplicationController(params) {

  var _spec = {
    containers: [{
        name:   'weavescope-app',
        image:  [ 'weaveworks/scope', params.tag ].join(':'), // overriding immage will be a thing
        args:   [ '--no-probe' ], // probably has useful flags users migth need, e.g. `--app.log.level=debug`
        ports:  [{ containerPort: 4040 }]
    }]
  };

  var _params = {
    replicas: 1
  }

  return makeResource('v1', 'ReplicationController', 'app', { spec: _spec }, _params);
}

exports.makeAppService = function appService(params) {
  var _params = {
    ports: [{ name: 'app', port: 80, targetPort: 4040, protocol: 'TCP' }]
  }

  if (_.isString(params.type)) {
    if (params.type === 'NodePort' || params.type === 'LoadBalancer') {
      _params.type = params.type;
    }
  }

  return makeResource('v1', 'Service', 'app', {}, _params);
}

exports.makeProbeDaemonSet = function probeDaemonSet(params) {

  if (_.isString(params.token)) {
    var probe_args = [ '--service-token', params.token ].join('=');
  } else {
    var probe_args = '$(WEAVESCOPE_APP_SERVICE_HOST):$(WEAVESCOPE_APP_SERVICE_PORT)';
  }

  var _spec = {
    hostPID: true,
    hostNetwork: true,
    containers: [{
        name:  'weavescope-probe',
        image: [ 'weaveworks/scope', params.tag ].join(':'),
        args:  [
          '--no-app',
          '--probe.docker.bridge=docker0', // may need to parametrised, some k8s installs use cbr0
          '--probe.docker=true',
          '--probe.kubernetes=true',
          // service token will got here, but user also might like to pass extra args
          probe_args
        ],
        securityContext: { privileged: true },
        resources: {
          limits: { cpu: '50m' } // would be great to see if we can build a live feedback loop here
        },
        volumeMounts: [{
            name: 'docker-sock',
            mountPath: '/var/run/docker.sock'
        }]
    }],
    volumes: [{
      name: 'docker-sock',
      hostPath: { path: '/var/run/docker.sock' }
    }]
  };

  return makeResource('extensions/v1beta1', 'DaemonSet', 'probe', { spec: _spec }, {});
}

exports.makeList = function list(components) {
  return { apiVersion: 'v1', kind: 'List', items: components };
}
