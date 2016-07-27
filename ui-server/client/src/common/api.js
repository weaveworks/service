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
    getOrganizations()
      .then(res => {
        if (res.organizations) {
          const instance = res.organizations.find(org => org.id === id);
          if (instance) {
            resolve(instance);
          } else {
            reject(Error('Instance not found'));
          }
        }
      })
      .catch(res => {
        if (res.status === 401) {
          hashHistory.push('/login');
        } else {
          const err = res.errors[0];
          trackException(err.message);
        }
      });
  });
}
