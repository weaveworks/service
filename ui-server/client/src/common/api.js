import { hashHistory } from 'react-router';

import { trackException } from './tracking';
import { encodeURIs, getData } from './request';

export function getNewInstanceId() {
  return getData('/api/users/generateOrgID');
}

export function getOrganizations() {
  const url = '/api/users/lookup';
  return getData(url);
}

export function getProbes(org) {
  if (org) {
    const url = encodeURIs`/api/app/${org}/api/probes`;
    return getData(url);
  }
  return Promise.reject();
}

export function getLogins() {
  const url = '/api/users/logins';
  return getData(url);
}

export function getInstance(id) {
  return new Promise((resolve, reject) => {
    // implies a cookie check
    getOrganizations()
      .then(res => {
        if (res.organizations) {
          const instance = res.organizations.find(org => org.id === id);
          if (instance) {
            // hack to only have one request
            instance.organizations = res.organizations;
            resolve(instance);
          } else {
            reject(Error('Instance not found'));
          }
        }
      })
      .catch(res => {
        // not logged in -> redirect
        if (res.status === 401) {
          hashHistory.push('/login');
        } else if (res.status === 403) {
          hashHistory.push('/login/forbidden');
        } else {
          const err = res.errors[0];
          trackException(err.message);
        }
      });
  });
}
