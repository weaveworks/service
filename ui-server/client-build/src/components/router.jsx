import React from 'react';

export default class RouterComponent extends React.Component {
  render() {
    return (
      <div id="container" style={{overflow: 'scroll', height: '100%'}}>
        {this.props.children}
      </div>
    );
  }
}
