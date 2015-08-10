import React from 'react';

export class Container extends React.Component {
  render() {

    let styles = {
      padding: '0 64px'
    };

    return (
      <div style={styles}>
        {this.props.children}
      </div>
    );
  }
}
