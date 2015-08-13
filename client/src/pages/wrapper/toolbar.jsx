import React from "react";
import { FlatButton, Toolbar, ToolbarGroup, ToolbarTitle } from "material-ui";
import { HashLocation } from "react-router";

export default class WrapperToolbar extends React.Component {

  render() {
    let styles = {
      toolbar: {
        width: '100%'
      }
    };

    return (
      <div style={styles.toolbar}>
        <Toolbar>
          <ToolbarGroup float="left">
            <ToolbarTitle text={this.props.organization} />
          </ToolbarGroup>
          <ToolbarGroup float="right">
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
