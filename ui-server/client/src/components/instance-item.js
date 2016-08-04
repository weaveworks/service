import React from 'react';
import MenuItem from 'material-ui/MenuItem';
import { hashHistory } from 'react-router';

import { encodeURIs } from '../common/request';

export default class InstanceItem extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.handleClick = this.handleClick.bind(this);
  }

  handleClick() {
    const url = encodeURIs`/instances/select/${this.props.id}`;
    hashHistory.push(url);
  }

  render() {
    return (
      <MenuItem primaryText={this.props.name} onClick={this.handleClick}
        style={{cursor: 'pointer'}} />
    );
  }
}
