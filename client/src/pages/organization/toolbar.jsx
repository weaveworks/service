import React from "react";
import { FlatButton, Toolbar, ToolbarGroup, ToolbarTitle } from "material-ui";
import { HashLocation } from "react-router";

export default class UserToolbar extends React.Component {

  render() {
    let styles = {
      toolbar: {
        width: '60%',
        float: 'right'
      }
    };

    return (
      <div style={styles.toolbar}>
        <Toolbar>
          <ToolbarGroup float="right">
            <ToolbarTitle text={this.props.user} />
            <FlatButton label="Logout" primary={true} onClick={this._handleTouchTap.bind(this)} />
          </ToolbarGroup>
        </Toolbar>
      </div>
    );
  }

  _handleTouchTap() {
    HashLocation.push('/logout');
  }
}
