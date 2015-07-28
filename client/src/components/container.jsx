import React from 'react';

export class Container extends React.Component {
  render() {

    let styles = {
      width: '960px',
      margin: '0 auto'
    };

    return (
      <div style={styles}>
        {this.props.children}
      </div>
    );
  }
}
