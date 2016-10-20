import React from 'react';
import MenuItem from 'material-ui/Menu';
import { Link } from 'react-router';

export default class MenuLink extends React.Component {
  render() {
    return (
      <MenuItem>
        <Link activeClassName="active" {...this.props} />
      </MenuItem>
    );
  }
}
