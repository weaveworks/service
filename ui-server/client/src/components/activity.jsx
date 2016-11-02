import React from 'react';
import CircularProgress from 'material-ui/CircularProgress';

export default class Activity extends React.Component {
  render() {
    const styles = Object.assign({
      textAlign: 'center',
      display: this.props.message ? 'block' : 'none',
      fontSize: '85%',
      opacity: 0.6
    }, this.props.style);

    return (
      <div style={styles}>
        <CircularProgress mode="indeterminate" />
        <p>{this.props.message}</p>
        {this.props.children}
      </div>
    );
  }
}
