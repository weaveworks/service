import React from 'react';

export default class PromMetricBrowserPrefix extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.handleClickPrefix = this.handleClickPrefix.bind(this);
  }

  handleClickPrefix() {
    this.props.onClickPrefix(this.props.prefix);
  }

  render() {
    const style = {
      cursor: 'pointer',
      textDecoration: 'underline',
      fontSize: '0.9em'
    };

    return (
      <span style={style} rel={this.props.prefix} onClick={this.handleClickPrefix}>
        {this.props.prefix} [{this.props.count}]
      </span>
    );
  }

}
