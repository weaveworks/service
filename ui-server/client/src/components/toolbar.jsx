import React from 'react';
import FontIcon from 'material-ui/FontIcon';
import Paper from 'material-ui/Paper';

import { encodeURIs } from '../common/request';
import Colors from '../common/colors';

import { Logo } from './logo';

export default class Toolbar extends React.Component {

  getLinks() {
    return [{
      iconClass: 'fa fa-cog',
      title: 'Settings for this instance',
      route: encodeURIs`#/org/${this.props.orgId}`
    }, {
      title: 'Manage instances',
      iconClass: 'fa fa-cubes',
      route: encodeURIs`#/instance/${this.props.orgId}`
    }, {
      title: 'User account',
      iconClass: 'fa fa-user',
      route: encodeURIs`#/account/${this.props.orgId}`
    }];
  }

  renderLinks() {
    return this.getLinks().map(link => {
      const linkColor = Colors.text3;
      const styles = {
        toolbarLink: {
          padding: 12,
          color: linkColor,
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

      return (
        <span style={styles.toolbarLinkWrapper} key={link.route}>
          <a style={styles.toolbarLink} title={link.title} href={link.route}>
            {link.iconClass && <FontIcon style={styles.toolbarLinkIcon} color={linkColor}
              hoverColor={Colors.text} className={link.iconClass} />}
            {link.label && <span style={styles.toolbarLinkLabel}>{link.label}</span>}
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
        display: 'flex',
        justifyContent: 'space-between',
        width: '100%'
      },
      toolbarCenter: {
        padding: 15
      },
      toolbarLeft: {
        top: 2,
        left: 12,
        padding: 8,
        width: 160,
        position: 'relative'
      },
      toolbarOrganization: {
        color: Colors.text2,
        fontSize: '110%',
        lineHeight: 1
      },
      toolbarOrganizationName: {
        color: Colors.text2,
        fontSize: '60%',
        lineHeight: 1,
        marginLeft: '0.5em'
      },
      toolbarRight: {
        padding: 15,
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
              <Logo />
            </div>
            <div style={styles.toolbarCenter}>
              {this.props.instance && <a href={encodeURIs`#/app/${this.props.orgId}`}
                style={styles.toolbarOrganization}>

                View Instance
                <span style={styles.toolbarOrganizationName}>
                  {this.props.instance.name}
                </span>
              </a>}
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
