import React from 'react';

export class Column extends React.Component {
  render() {
    const width = this.props.width ? this.props.width : '45%';

    const styles = {
      float: 'left',
      margin: '0 36px',
      width: `calc(${width} - 72px)`
    };

    return (
      <div style={styles}>
        {this.props.children}
      </div>
    );
  }
}
