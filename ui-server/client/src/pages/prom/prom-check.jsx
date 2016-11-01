/* eslint react/jsx-no-bind:0 */
import React from 'react';
import { browserHistory } from 'react-router';
import { connect } from 'react-redux';

import { encodeURIs } from '../../common/request';
import Activity from '../../components/activity';

export class PromCheck extends React.Component {
  componentWillReceiveProps(nextProps) {
    const { instance, params } = nextProps;
    // routing based on prometheus metrics seen
    if (instance && typeof instance.prometheusJobs !== 'undefined') {
      if (instance.prometheusJobs !== null) {
        // we got data, route to wrapper
        const url = encodeURIs`/prom/${params.orgId}/wrapper`;
        browserHistory.push(url);
      } else {
        // redirect to setup page if no prometheus data found
        const url = encodeURIs`/prom/${params.orgId}/setup`;
        browserHistory.push(url);
      }
    }
  }

  render() {
    const activityText = 'Checking for Prometheus data...';

    return (
      <Activity message={activityText} style={{marginTop: 64}} />
    );
  }
}

function mapStateToProps(state, ownProps) {
  return {
    instance: state.instances[ownProps.params.orgId]
  };
}

export default connect(mapStateToProps)(PromCheck);
