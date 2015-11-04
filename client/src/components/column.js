import React from 'react';

export class Column extends React.Component {
  render() {
    const styles = {
      margin: '0 36px'
    };

    if (this.props.width) {
      styles.width = this.props.width;
    } else {
      styles.flex = 1;
      if (this.props.minWidth) {
        styles.minWidth = this.props.minWidth;
      }
    }

    return (
      <div style={styles}>
        {this.props.children}
      </div>
    );
  }
}
