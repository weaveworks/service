import React from 'react';

export class Column extends React.Component {
  render() {

    let styles = {
      float: 'left',
      marginRight: 48,
      width: '45%'
    };

    return (
      <div style={styles}>
        {this.props.children}
      </div>
    );
  }
}
