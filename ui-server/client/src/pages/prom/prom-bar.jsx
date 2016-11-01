import React from 'react';
import { browserHistory } from 'react-router';
import { connect } from 'react-redux';

import { encodeURIs } from '../../common/request';
import Colors from '../../common/colors';
import Box from '../../components/box';
import Column from '../../components/column';
import FlexContainer from '../../components/flex-container';
import PromStatus from './prom-status';
import PromMetricBrowser from './prom-metric-browser';
import PromSystemQueries from './prom-system-queries';

const NODE_QUERIES = [{
  label: 'CPU usage in % by mode',
  query: 'sum by (mode) (irate(node_cpu{job="node",mode!="idle"}[5m]))'
}, {
  label: 'Disk IO (% time spent) by device',
  query: 'irate(node_disk_io_time_ms{job="node"}[5m]) / 1000 * 100'
}, {
  label: 'Free memory in bytes',
  query: 'node_memory_MemFree{job="node"} + node_memory_Buffers{job="node"}'
    + ' + node_memory_Cached{job="node"}'
}, {
  label: 'Network received bytes/s by device',
  query: 'irate(node_network_receive_bytes{job="node",device!="lo"}[5m])'
}, {
  label: 'Filesystem fullness in % by filesystem',
  query: '100 - (node_filesystem_free{job="node"} / node_filesystem_size{job="node"} * 100)'
}];

const K8S_QUERIES = [{
  label: 'CPU usage in % by pod',
  query: 'sum(irate(container_cpu_usage_seconds_total{job="kubernetes-nodes",'
    + 'io_kubernetes_pod_namespace="monitoring",io_kubernetes_pod_name=~"prometheus-.*"}[1m]))'
    + ' by (io_kubernetes_pod_namespace,io_kubernetes_pod_name)'
}, {
  label: 'Memory usage by pod',
  query: 'sum(container_memory_usage_bytes{job="kubernetes-nodes",'
    + 'io_kubernetes_pod_namespace="monitoring",io_kubernetes_pod_name=~"prometheus-.*"})'
    + ' by (io_kubernetes_pod_namespace,io_kubernetes_pod_name)'
}];

const NET_QUERIES = [{
  label: 'IP address space exhaustion in %',
  query: 'scalar(sum(weave_ips{state="local-used"}))'
    + ' / scalar(topk(1, weave_ips{state="total"})) * 100'
}, {
  label: 'Number of local DNS entries per each host',
  query: 'weave_dns_entries{state="local"}'
}, {
  label: 'Connection termination rate per second',
  query: 'sum(rate(weave_connection_termination_count[5m]))'
}, {
  label: 'Number of blocked connections per transport-layer protocol',
  query: 'sum(weavenpc_blocked_connections_total) by (protocol)'
}, {
  label: 'Frequent protocol-dport combinations of blocked connections',
  query: 'topk(10, sum(weavenpc_blocked_connections_total) by (protocol, dport))'
}];

const SYSTEM_QUERIES = [{
  prefix: 'node',
  label: 'Nodes',
  queries: NODE_QUERIES
}, {
  prefix: 'k8s',
  label: 'Kubernetes',
  queries: K8S_QUERIES
}, {
  prefix: 'weave',
  label: 'Weave Net',
  queries: NET_QUERIES
}];

const makeDocumentationItems = (orgId) => [{
  text: 'Set up Prometheus',
  relativeHref: encodeURIs`/prom/${orgId}/setup`,
  description: 'Steps to set up a local Prometheus that sends data to Weave Cloud'
}, {
  text: 'Prometheus Query Examples',
  href: 'https://prometheus.io/docs/querying/examples/',
  description: 'Learn about the flexible query language to leverage things like dimensionality'
}];

function renderDocumentation(items) {
  const style = {
    icon: {
      textDecoration: 'underline',
    },
    item: {
      cursor: 'pointer',
      textDecoration: 'underline',
      fontSize: '0.9em'
    }
  };
  return items.map(({action, description, href, relativeHref, text}) => {
    const Tag = href ? 'a' : 'span';
    const handleClick = relativeHref ? () => {
      browserHistory.push(relativeHref);
    } : action;

    return (
      <div key={href || relativeHref || text}>
        <Tag onClick={handleClick} target={href} href={href} style={style.item}
          title={description}>
          {text}
          &nbsp;
          {href && <span className="fa fa-external-link" style={style.icon} />}
        </Tag>
      </div>
    );
  });
}

export class PromBar extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      metricPrefix: '',
      systemQueries: SYSTEM_QUERIES
    };

    this.handleClickSystemQuery = this.handleClickSystemQuery.bind(this);
    this.handleClickClearPrefix = this.handleClickClearPrefix.bind(this);
    this.handleClickMetricPrefix = this.handleClickMetricPrefix.bind(this);
  }

  renderSystemQueries(systemQueries) {
    const style = {
      cursor: 'pointer',
      textDecoration: 'underline',
      fontSize: '0.9em'
    };
    return systemQueries.map(({label, query}) => <div
      onClick={this.handleClickSystemQuery} style={style} title={query}>{label}</div>);
  }

  handleClickSystemQuery(ev) {
    ev.preventDefault();
    const query = ev.target.title;
    this.props.setExpressionField(query);
    this.props.clickFrameExecuteButton();
  }

  handleClickMetricPrefix(nextPrefix) {
    let { metricPrefix } = this.state;
    metricPrefix = `${metricPrefix}${nextPrefix}`;
    this.setState({ metricPrefix });
    this.props.setExpressionField(metricPrefix);
  }

  handleClickClearPrefix() {
    this.setState({ metricPrefix: '' });
  }

  render() {
    const { instance } = this.props;
    const { activeCategory } = this.state;
    const metricNames = instance ? instance.prometheusMetricNames || [] : [];
    const styles = {
      categories: {
        display: 'flex',
      },
      container: {
        fontSize: '0.9em',
        padding: '16px 0 8px'
      },
      documentation: {
        borderTop: `1px dotted ${Colors.text4}`,
        padding: '8px 16px',
      },
      heading: {
        color: Colors.text3,
        textTransform: 'uppercase',
        fontSize: '0.8em',
        marginBottom: 4,
        marginTop: 4
      },
    };

    return (
      <FlexContainer style={styles.container}>
        <Column style={{flex: 2}}>
          <div style={styles.heading}>Prometheus System Queries</div>
          <div style={styles.categories}>
            {this.state.systemQueries.map(sq => <PromSystemQueries
              key={sq.prefix}
              onClickCategory={this.handleClickCategory}
              onClickQuery={this.handleClickSystemQuery}
              active={activeCategory}
              {...sq} />)}
          </div>
        </Column>
        <Column style={{flex: 1, margin: '0'}}>
          <div style={styles.heading}>Detected Metric Names</div>
          <PromMetricBrowser
            onClickClearPrefix={this.handleClickClearPrefix}
            onClickMetricPrefix={this.handleClickMetricPrefix}
            metrics={metricNames}
            prefix={this.state.metricPrefix} />
        </Column>
        <Column width={220}>
          <Box>
            <PromStatus orgId={this.props.orgId} />
            <div style={styles.documentation}>
              {renderDocumentation(makeDocumentationItems(this.props.orgId))}
            </div>
          </Box>
        </Column>
      </FlexContainer>
    );
  }
}

function mapStateToProps(state, ownProps) {
  return {
    instance: state.instances[ownProps.orgId]
  };
}

export default connect(mapStateToProps)(PromBar);
