import React from 'react';

import Colors from '../../common/colors';
import PromMetricBrowserPrefix from './prom-metric-browser-prefix';

export const DELIMITER = '_';

export function getNextNames(metrics, prefix) {
  let names = metrics;

  // remove current prefix
  if (prefix) {
    names = names
      .filter(name => name.indexOf(prefix) === 0)
      .map(name => name.substr(prefix.length + 1));
  }

  // find next names
  const namesMap = {};
  names.forEach(name => {
    const nextPrefix = name.split(DELIMITER)[0];
    if (nextPrefix) {
      if (!namesMap[nextPrefix]) {
        namesMap[nextPrefix] = [];
      }
      namesMap[nextPrefix] = [...namesMap[nextPrefix], name];
    }
  });

  return namesMap;
}

export default class PromMetricBrowser extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.handleClickPrefix = this.handleClickPrefix.bind(this);
  }

  handleClickPrefix(prefix) {
    this.props.onClickMetricPrefix(prefix);
  }

  render() {
    const { metrics, prefixes, onClickClearPrefix } = this.props;
    const prefix = prefixes.join(DELIMITER);
    const names = getNextNames(metrics, prefix);
    const styles = {
      container: {
        marginLeft: 24
      },
      prefixes: {
        marginTop: 4,
        marginBottom: 4,
        textTransform: 'uppercase',
        color: Colors.text2,
        fontSize: '0.9em',
      },
      close: {
        cursor: 'pointer',
        marginLeft: '0.5em'
      }
    };

    return (
      <div style={styles.container}>
         <div style={styles.prefixes}>
          {prefix || 'Prefixes'}
          {prefix && <span className="fa fa-close" style={styles.close}
            onClick={onClickClearPrefix} />}
        </div>
        <div>
          {Object.keys(names).map((name, index, arr) => (
            <span key={name}>
              <PromMetricBrowserPrefix
                prefix={name}
                count={names[name].length}
                onClickPrefix={this.handleClickPrefix} />
              {index < arr.length - 1 && ', '}
            </span>
          ))}
        </div>
      </div>
    );
  }

}
