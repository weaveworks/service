/* eslint react/jsx-no-bind:0 */
import React from 'react';
import { connect } from 'react-redux';
import _get from 'lodash/get';

import { getPrometheusMetricNames, getPrometheusQuery } from '../../common/api';
import { receivePrometheusError, receivePrometheusInstances, receivePrometheusJobs,
  receivePrometheusMetricNames } from '../../actions';
import { trackView } from '../../common/tracking';
import PrivatePage from '../../components/private-page';


// Singleton status timer that makes sure to dispatch only when instance is unchanged
let currentOrg = null;
let checkPrometheusTimer = null;
const TIMER_DELAY = 10 * 1000;

/**
 * Run a couple of status queries in series
 */
function checkPrometheus(orgId, dispatch) {
  clearTimeout(checkPrometheusTimer);
  currentOrg = orgId;

  getPrometheusMetricNames(orgId)
    .then(res => {
      if (orgId === currentOrg) {
        dispatch(receivePrometheusMetricNames(orgId, res.data));
      }
    })
    .then(() => getPrometheusQuery(orgId, 'count(count by (job)(up))'))
    .then(res => {
      if (orgId === currentOrg) {
        dispatch(receivePrometheusJobs(orgId, _get(res, ['data', 'result', 0, 'value', 1])));
      }
    })
    .then(() => getPrometheusQuery(orgId, 'count(count by (instance)(up))'))
    .then(res => {
      if (orgId === currentOrg) {
        dispatch(receivePrometheusInstances(orgId, _get(res, ['data', 'result', 0, 'value', 1])));
      }
    })
    .catch(() => {
      if (orgId === currentOrg) {
        dispatch(receivePrometheusError(orgId));
      }
    })
    .then(() => {
      checkPrometheusTimer = setTimeout(() => {
        checkPrometheus(orgId, dispatch);
      }, TIMER_DELAY);
    });
}


export class PromPage extends React.Component {

  componentDidMount() {
    trackView('Prom');
    checkPrometheus(this.props.params.orgId, this.props.dispatch);
  }

  componentWillUnmount() {
    clearTimeout(checkPrometheusTimer);
  }

  render() {
    return (
      <PrivatePage page="prom" {...this.props.params}>
        {this.props.children}
      </PrivatePage>
    );
  }
}

export default connect()(PromPage);
