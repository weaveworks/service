import _ from 'lodash';
import * as api from '../common/api';

import { trackException } from '../common/tracking';


const ACTION_TYPES = [
  'RECEIVE_ORGANIZATION_DATA',
  'RECEIVE_ORGANIZATIONS',
  'UPDATE_INSTANCE',
];


export const ActionTypes = _.zipObject(ACTION_TYPES, ACTION_TYPES);

//
// Actions!
//

export function updateInstance(instance) {
  return {
    type: ActionTypes.UPDATE_INSTANCE,
    instance
  };
}


//
// getData from the server Actions!
//


export function getOrganizationData(id) {
  return (dispatch) => {
    api.getOrganizationData(id)
      .then(org => {
        dispatch({
          type: ActionTypes.RECEIVE_ORGANIZATION_DATA,
          org
        });
      })
      .catch(trackException);
  };
}


export function getOrganizations() {
  return (dispatch) => {
    api.getOrganizations().then(res => {
      dispatch({
        type: ActionTypes.RECEIVE_ORGANIZATIONS,
        organizations: res.organizations,
        email: res.email,
      });
    });
  };
}


export function getInstance(id) {
  return (dispatch) => {
    api.getInstance(id)
      .then(res => {
        dispatch({
          type: ActionTypes.RECEIVE_ORGANIZATIONS,
          organizations: res.organizations,
          email: res.email,
        });
      })
      .catch(trackException);
  };
}
