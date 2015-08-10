import React from 'react';

export class Column extends React.Component {
  render() {

    let styles = {
      float: 'left',
      margin: '0 2%',
      width: '45%'
    };

    return (
      <div style={styles}>
        {this.props.children}
      </div>
    );
  }
}
