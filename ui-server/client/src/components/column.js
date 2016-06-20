import React from 'react';

export class Column extends React.Component {
  render() {
    const styles = Object.assign({
      margin: '0 36px'
    }, this.props.style);

    if (this.props.width) {
      styles.width = `${this.props.width}px`;
    } else {
      styles.flex = 1;
      if (this.props.minWidth) {
        styles.minWidth = `${this.props.minWidth}px`;
      }
    }

    return (
      <div style={styles}>
        {this.props.children}
      </div>
    );
  }
}
