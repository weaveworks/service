import React from "react";
import { Tabs, FlatButton, Toolbar, ToolbarGroup, ToolbarTitle } from "material-ui";
import { HashLocation } from "react-router";

export default class WrapperToolbar extends React.Component {

  getLinks() {
    return [{
      label: 'Service',
      route: `#/org/${this.props.organization}`
    }, {
      label: 'Scope',
      route: `#/app/${this.props.organization}`
    }];
  }

  renderLinks() {
    return this.getLinks().map(link => {
      const isOnPage = link.route === window.location.hash;
      return (
        <FlatButton label={link.label} secondary={isOnPage}
          onClick={this._handleLinkClick.bind(this, link.route)} />
      );
    }.bind(this));
  }

  render() {
    let styles = {
      toolbar: {
        width: '100%'
      }
    };

    const links = this.renderLinks();

    return (
      <div style={styles.toolbar}>
        <Toolbar>
          <ToolbarGroup float="left">
            <ToolbarTitle text={'Organization: ' + this.props.organization} />
            {links}
          </ToolbarGroup>
          <ToolbarGroup float="right">
            <ToolbarTitle text={this.props.user} />
            <FlatButton label="Logout" primary={true} onClick={this._handleTouchTap.bind(this)} />
          </ToolbarGroup>
        </Toolbar>
      </div>
    );
  }

  _handleLinkClick(route) {
    HashLocation.push(route);
  }

  _handleTouchTap() {
    window.location.href = '/';
  }
}
