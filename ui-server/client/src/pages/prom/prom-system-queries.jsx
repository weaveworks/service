import React from 'react';

import Colors from '../../common/colors';

export default class PromSystemQueries extends React.Component {

  render() {
    const { label, metricNames, prefix, queries,
      onClickCategory, onClickQuery, onClickSetup } = this.props;

    const isAvailable = metricNames.some(name => name.indexOf(prefix) === 0);

    const styles = {
      category: {
        marginLeft: 24,
      },
      label: {
        marginTop: 4,
        marginBottom: 4,
        color: Colors.text2,
        textTransform: 'uppercase',
        fontSize: '0.9em',
      },
      prefix: {
        textTransform: 'capitalize'
      },
      query: {
        fontSize: '0.9em',
        textDecoration: 'underline',
        paddingBottom: 4,
        cursor: 'pointer'
      },
      setupLink: {
        marginTop: 2,
        textDecoration: 'underline',
        paddingBottom: 4,
        cursor: 'pointer'
      },
      unavailable: {
        width: '180px',
        display: 'flex',
        flexWrap: 'wrap',
      },
      unavailableHint: {
        fontSize: '0.9em',
        marginTop: 4,
        textTransform: 'none',
      },
      unavailableQuery: {
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        opacity: 0.33,
        fontSize: '0.7em',
        paddingBottom: 4
      }
    };

    return (
      <div style={styles.category}>
        <div onClick={onClickCategory} style={styles.label}>
          {label}
          {!isAvailable && <div style={styles.unavailableHint}>
            <span style={styles.prefix}>{prefix}</span> metrics are not available yet.
            <div onClick={onClickSetup} style={styles.setupLink}>
            Configure system queries</div>
          </div>}
        </div>
        {isAvailable && <div>
          {queries.map(({ label: text, query }) => <div
            style={styles.query}
            onClick={onClickQuery}
            title={query}
            key={text}>{text}</div>)}
        </div>}
        {!isAvailable && <div style={styles.unavailable}>
          {queries.map(({ label: text }) => <span
            style={styles.unavailableQuery}
            title={text}
            key={text}>{text}</span>)}
        </div>}
      </div>
    );
  }

}
