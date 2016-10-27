import React from 'react';
import Paper from 'material-ui/Paper';
import Menu from 'material-ui/Menu';
import Subheader from 'material-ui/Subheader';

export default class MenuSection extends React.Component {
  render() {
    return (
      <Paper style={{marginTop: '4em', marginBottom: '1em'}}>
        <Menu>
          {this.props.title && <Subheader>{this.props.title}</Subheader>}
          {this.props.children}
        </Menu>
      </Paper>
    );
  }
}
