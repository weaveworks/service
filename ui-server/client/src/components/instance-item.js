import React from 'react';
import MenuItem from 'material-ui/MenuItem';
import { browserHistory } from 'react-router';

import { encodeURIs } from '../common/request';

function addUrlState(url) {
  const { hash } = window.location;
  if (hash) {
    return `${url}${hash}`;
  }
  return url;
}

export default class InstanceItem extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.handleClick = this.handleClick.bind(this);
  }

  handleClick() {
    const url = encodeURIs`/app/${this.props.id}`;
    browserHistory.push(addUrlState(url));
  }

  render() {
    return (
      <MenuItem primaryText={this.props.name} onClick={this.handleClick}
        style={{cursor: 'pointer'}} />
    );
  }
}
