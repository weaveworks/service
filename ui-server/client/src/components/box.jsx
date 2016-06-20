import React from 'react';

export class Box extends React.Component {
  render() {
    const styles = Object.assign({
      border: 'solid 1px #d9d9d9'
    }, this.props.style);

    return (
      <div style={styles}>
        {this.props.children}
      </div>
    );
  }
}
