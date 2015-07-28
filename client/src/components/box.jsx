import React from 'react';

export class Box extends React.Component {
  render() {

    let styles = {
      border: 'solid 1px #d9d9d9'
    };

    return (
      <div style={styles}>
        {this.props.children}
      </div>
    );
  }
}
