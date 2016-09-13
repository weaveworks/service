import zipObject from 'lodash/zipObject';
import * as api from '../common/api';

import { trackException } from '../common/tracking';


const ACTION_TYPES = [
  'FOCUS_FRAME',
  'RECEIVE_ORGANIZATION_DATA',
  'RECEIVE_ORGANIZATIONS',
  'RECEIVE_ORGANIZATION_USERS',
  'REQUEST_INSTANCES_MENU_CHANGE',
  'UPDATE_INSTANCE',
  'UPDATE_SCOPE_VIEW_STATE',
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

export function focusFrame() {
  return {
    type: ActionTypes.FOCUS_FRAME,
  };
}

export function requestInstancesMenuChange(open) {
  return {
    type: ActionTypes.REQUEST_INSTANCES_MENU_CHANGE,
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


export function getOrganizationUsers(id) {
  return (dispatch) => {
    api.getOrganizationUsers(id)
      .then(res => {
        dispatch({
          type: ActionTypes.RECEIVE_ORGANIZATION_USERS,
          users: res.users,
          orgId: id,
        });
      })
      .catch(trackException);
  };
}
