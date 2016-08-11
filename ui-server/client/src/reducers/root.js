import debug from 'debug';
import { ActionTypes } from '../actions';


const error = debug('service:error');


//
// current instance is specified in the route, one day we might be able to sync that in here with
// one of the many router-redux integrations
//
export const initialState = {
  email: '',
  instances: {},
};


function mergeInstances(instances, newInstances = []) {
  const instancesCopy = Object.assign({}, instances);

  newInstances.forEach(instance => {
    instancesCopy[instance.id] = Object.assign({}, instancesCopy[instance.id], instance);
  });

  return instancesCopy;
}


export function rootReducer(state = initialState, action) {
  if (!action.type) {
    error('Payload missing a type!', action);
  }

  switch (action.type) {

    case ActionTypes.UPDATE_INSTANCE: {
      return Object.assign({}, state, {
        instances: mergeInstances(state.instances, [action.instance]),
      });
    }

    //
    // RECEIVE the things
    //

    case ActionTypes.RECEIVE_ORGANIZATION_DATA: {
      return Object.assign({}, state, {
        email: action.org.user,
        instances: mergeInstances(state.instances, [{
          id: action.org.id,
          name: action.org.name,
          probeToken: action.org.probeToken,
        }]),
      });
    }

    case ActionTypes.RECEIVE_ORGANIZATIONS: {
      return Object.assign({}, state, {
        email: action.email,
        instances: mergeInstances(state.instances, action.organizations),
      });
    }

    default: {
      error('Why are we here at default?');
      return state;
    }
  }
}
