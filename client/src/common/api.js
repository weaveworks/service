import { getData } from './request';


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

export function getLogins() {
  const url = '/api/users/logins';
  return getData(url);
}
