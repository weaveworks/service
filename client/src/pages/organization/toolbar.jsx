import React from "react";
import { Styles, FlatButton, Toolbar, ToolbarGroup, ToolbarTitle } from "material-ui";

const ThemeManager = new Styles.ThemeManager();

export default class UserToolbar extends React.Component {

  static childContextTypes = {
    muiTheme: React.PropTypes.object
  }

  getChildContext() {
    return {
      muiTheme: ThemeManager.getCurrentTheme()
    };
  }

  render() {
    let styles = {
      toolbar: {
        width: '66%',
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
    window.location.href = '/';
  }
}
