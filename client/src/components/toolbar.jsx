import React from 'react';
import { Paper } from 'material-ui';

import Colors from '../common/colors';

export default class WrapperToolbar extends React.Component {

  getLinks() {
    return [{
    }, {
      title: 'Visit my Scope Instance',
      label: 'My Scope',
      route: `#/app/${this.props.organization}`
    }, {
      iconClass: 'fa fa-cog',
      title: 'Settings',
      route: `#/org/${this.props.organization}`
    }, {
      iconClass: 'fa fa-sign-out',
      title: `Log out ${this.props.user}`,
      route: `#/logout`
    }];
  }

  renderLinks() {
    const styles = {
      toolbarLink: {
        padding: 12
      },
      toolbarLinkIcon: {
        fontSize: '130%'
      },
      toolbarLinkLabel: {
        fontSize: '110%'
      },
      toolbarLinkWrapper: {
        padding: 0
      }
    };

    return this.getLinks().map(link => {
      const isOnPage = link.route === window.location.hash;
      const linkClass = isOnPage ? 'active' : '';
      return (
        <span style={styles.toolbarLinkWrapper}>
          <a style={styles.toolbarLink} title={link.title} className={linkClass} href={link.route}>
            <span style={styles.toolbarLinkLabel}>{link.label}</span>
            <span style={styles.toolbarLinkIcon} className={link.iconClass} />
          </a>
        </span>
      );
    });
  }

  render() {
    const styles = {
      filler: {
        height: 50,
      },
      toolbar: {
        width: '100%'
      },
      toolbarLeft: {
        float: 'left',
        padding: '16px 24px'
      },
      toolbarOrganization: {
        color: Colors.text2,
        fontSize: '110%',
        lineHeight: 1
      },
      toolbarOrganizationLabel: {
        color: Colors.text2,
        fontSize: '60%',
        lineHeight: 1,
        marginRight: '0.5em',
        textTransform: 'uppercase'
      },
      toolbarRight: {
        float: 'right',
        padding: '12px 24px'
      },
      toolbarWrapper: {
        backgroundColor: '#e4e4ed',
        position: 'absolute',
        width: '100%'
      }
    };

    const links = this.renderLinks();

    return (
      <div>
        <Paper zDepth={1} style={styles.toolbarWrapper}>
          <div style={styles.toolbar}>
            <div style={styles.toolbarLeft}>
              <span style={styles.toolbarOrganizationLabel}>
                App
              </span>
              <span style={styles.toolbarOrganization}>
                {this.props.organization}
              </span>
            </div>
            <div style={styles.toolbarRight}>
              {links}
            </div>
          </div>
        </Paper>
        <div style={styles.filler} />
      </div>
    );
  }
}
