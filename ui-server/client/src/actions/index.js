import zipObject from 'lodash/zipObject';
import * as api from '../common/api';

import { trackException } from '../common/tracking';


const ACTION_TYPES = [
  'RECEIVE_ORGANIZATION_DATA',
  'RECEIVE_ORGANIZATIONS',
  'UPDATE_INSTANCE',
  'UPDATE_SCOPE_VIEW_STATE',
  'UPDATE_INSTANCES_MENU_OPEN',
];


export const ActionTypes = zipObject(ACTION_TYPES, ACTION_TYPES);

//
// Actions!
//

export function updateInstance(instance) {
  return {
    type: ActionTypes.UPDATE_INSTANCE,
    instance
  };
}

export function updateScopeViewState(scopeViewState) {
  return {
    type: ActionTypes.UPDATE_SCOPE_VIEW_STATE,
    scopeViewState
  };
}

export function updateInstancesMenuOpen(open) {
  return {
    type: ActionTypes.UPDATE_INSTANCES_MENU_OPEN,
    open
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
