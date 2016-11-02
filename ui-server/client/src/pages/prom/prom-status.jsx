import React from 'react';
import { connect } from 'react-redux';

import Metric from '../../components/metric';
import PromConnection from './prom-connection';

class PromStatus extends React.Component {

  render() {
    const styles = {
      connection: {
        marginBottom: 4
      },
      status: {
        padding: '12px 16px'
      },
      statusItem: {
        display: 'inline-block',
        marginRight: 4,
        marginTop: 4
      },
    };

    const { instances, jobs, connected } = this.props;


    return (
      <div style={styles.status}>
        <PromConnection style={styles.connection} connected={connected} />
        {/* <Metric label="Ingest rate" value={ingestRate} unit="KB/s" /> */}
        <Metric label="Instances" value={instances} style={styles.statusItem} />
        <Metric label="Jobs" value={jobs} style={styles.statusItem} />
      </div>
    );
  }
}

function mapStateToProps(state, ownProps) {
  const instance = state.instances[ownProps.orgId];
  const jobs = instance && instance.prometheusJobs ? instance.prometheusJobs : 'n/a';
  const instances = instance
    && instance.prometheusInstances ? instance.prometheusInstances : 'n/a';
  const connected = instance && (instance.prometheusJobs || instances.prometheusInstances);

  return { connected, instances, jobs };
}

export default connect(mapStateToProps)(PromStatus);
