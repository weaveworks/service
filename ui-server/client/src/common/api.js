import { browserHistory } from 'react-router';

import { trackException } from './tracking';
import { encodeURIs, getData } from './request';

export function getNewInstanceId() {
  return getData('/api/users/generateOrgID');
}

export function getOrganizations() {
  const url = '/api/users/lookup';
  return getData(url);
}

export function getOrganizationData(id) {
  const url = encodeURIs`/api/users/org/${id}`;
  return getData(url);
}

export function getOrganizationUsers(id) {
  const url = encodeURIs`/api/users/org/${id}/users`;
  return getData(url);
}

export function getProbes(org) {
  if (org) {
    const url = encodeURIs`/api/app/${org}/api/probes`;
    return getData(url);
  }
  return Promise.reject();
}

export function getPrometheusMetricNames(org) {
  if (org) {
    const url = encodeURIs`/api/app/${org}/api/prom/api/v1/label/__name__/values`;
    return getData(url);
  }
  return Promise.reject();
}

export function getPrometheusQuery(org, query) {
  if (org) {
    const url = encodeURIs`/api/app/${org}/api/prom/api/v1/query?query=${query}`;
    return getData(url);
  }
  return Promise.reject();
}

export function getLogins() {
  const url = '/api/users/logins';
  return getData(url);
}

export function getInstance(id) {
  return new Promise((resolve) => {
    // implies a cookie check
    getOrganizations()
      .then(res => {
        const { organizations, email } = res;
        const instance = organizations && organizations.find(org => org.id === id);
        if (instance) {
          // include all to only have one request
          resolve({
            email,
            organizations,
            instance
          });
        } else {
          browserHistory.push('/instances/error/notfound');
        }
      })
      .catch(res => {
        // not logged in -> redirect
        if (res.status === 401) {
          browserHistory.push('/login');
        } else if (res.status === 403) {
          browserHistory.push('/login/forbidden');
        } else {
          const err = res.errors[0];
          trackException(err.message);
        }
      });
  });
}
