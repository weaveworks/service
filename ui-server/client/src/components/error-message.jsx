import React from 'react';

import { amber900 } from 'material-ui/styles/colors';

export default class ErrorMessage extends React.Component {
  render() {
    const { hidden, message, style } = this.props;

    const rootStyle = Object.assign({
      justifyContent: 'center',
      display: message && !hidden ? 'flex' : 'none'
    }, style);

    const styles = {
      error: {
        width: '50%',
        margin: 16,
        display: 'inline-block',
        position: 'relative',
        fontSize: 14,
      },

      errorIcon: {
        position: 'absolute',
        top: 0,
        left: -2,
        fontSize: 32,
        color: amber900
      },

      errorLabel: {
        color: amber900,
        textAlign: 'left',
        paddingLeft: 32
      },

    };

    return (
      <div style={rootStyle}>
        <div style={styles.error}>
          <span className="fa fa-ban" style={styles.errorIcon}></span>
          <div style={styles.errorLabel}>
            {message}
          </div>
        </div>
      </div>
    );
  }
}
