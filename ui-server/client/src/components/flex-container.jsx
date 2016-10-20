import React from 'react';

export default class FlexContainer extends React.Component {
  render() {
    const styles = Object.assign({
      display: 'flex',
      flexDirection: 'row',
      flexWrap: 'wrap',
      justifyContent: 'space-around',
      alignContent: 'flex-start',
      alignItems: 'flex-start',
      padding: '0 64px'
    }, this.props.style);

    return (
      <div style={styles}>
        {this.props.children}
      </div>
    );
  }
}
