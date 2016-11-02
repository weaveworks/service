import React from 'react';

export default class Metric extends React.Component {
  render() {
    const styles = Object.assign({
      labelStyle: {
        marginRight: '0.5em'
      },
      unitStyle: {
        marginLeft: '0.25em'
      }
    }, this.props.style);

    return (
      <div style={styles}>
        <span style={styles.labelStyle}>{this.props.label}:</span>
        <span style={styles.valueStyle}>{this.props.value}</span>
        <span style={styles.unitStyle}>{this.props.unit}</span>
      </div>
    );
  }
}
