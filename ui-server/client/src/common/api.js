import { encodeURIs, getData } from './request';


export function getOrganizations() {
  const url = '/api/users/lookup';
  return getData(url).then(resp => {
    //
    // --
    // FIXME: DEPRECATED
    // Handle backwards-compatibility while deploying
    //
    if (resp.organizationName) {
      resp.organizations = [{
        name: resp.organizationName,
        firstProbeUpdateAt: resp.firstProbeUpdateAt
      }];
    }
    // ---
    //

    return resp;
  });
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
