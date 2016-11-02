import React from 'react';
import { green400, red400 } from 'material-ui/styles/colors';

export default class PromConnection extends React.Component {

  render() {
    const styles = {
      iconOff: {
        position: 'relative',
        top: 1,
        display: 'inline-block',
        border: `2px solid ${red400}`,
        height: 8,
        width: 8,
        borderRadius: '50%'
      },
      iconOn: {
        display: 'inline-block',
        backgroundColor: green400,
        height: 9,
        width: 9,
        borderRadius: '50%'
      },
      label: {
        marginLeft: 6
      }
    };

    const text = this.props.connected ? 'Prometheus connected' : 'No Prometheus found';
    const iconStyle = this.props.connected ? styles.iconOn : styles.iconOff;

    return (
      <div style={this.props.style}>
        <span style={iconStyle}></span>
        <span style={styles.label}>{text}</span>
      </div>
    );
  }

}
