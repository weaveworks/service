import React from 'react';
import Divider from 'material-ui/Divider';
import FlatButton from 'material-ui/FlatButton';
import FontIcon from 'material-ui/FontIcon';
import IconButton from 'material-ui/IconButton';
import IconMenu from 'material-ui/IconMenu';
import MenuItem from 'material-ui/MenuItem';
import Paper from 'material-ui/Paper';
import { hashHistory } from 'react-router';

import { encodeURIs } from '../common/request';
import Colors from '../common/colors';

import InstanceItem from './instance-item';
import { Logo } from './logo';

export default class Toolbar extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.handleClickInstance = this.handleClickInstance.bind(this);
    this.handleManageInstances = this.handleManageInstances.bind(this);
  }

  handleClickInstance() {
    const url = encodeURIs`/app/${this.props.orgId}`;
    hashHistory.push(url);
  }

  handleManageInstances() {
    const url = encodeURIs`/instance/${this.props.orgId}`;
    hashHistory.push(url);
  }

  getLinks() {
    return [{
      iconClass: 'fa fa-cog',
      title: 'Settings for this instance',
      route: encodeURIs`#/org/${this.props.orgId}`
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
        padding: 8
      },
      toolbarLeft: {
        top: 2,
        left: 12,
        padding: 8,
        width: 160,
        position: 'relative'
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
    const viewText = this.props.instanceName ? `View ${this.props.instanceName}` : 'Loading...';

    return (
      <div>
        <Paper zDepth={1} style={styles.toolbarWrapper}>
          <div style={styles.toolbar}>
            <div style={styles.toolbarLeft}>
              <Logo />
            </div>
            <div style={styles.toolbarCenter}>
              <div style={{position: 'relative'}}>
                <FlatButton
                  style={{color: Colors.text2}}
                  onClick={this.handleClickInstance}
                  label={viewText} />
                {this.props.instances && this.props.instances.length > 0 && <IconMenu
                  iconButtonElement={<IconButton iconStyle={{color: Colors.text2}}>
                    <FontIcon className="fa fa-caret-down" />
                  </IconButton>}
                  style={{position: 'absolute', right: -40, top: -6}}
                  anchorOrigin={{horizontal: 'right', vertical: 'top'}}
                  targetOrigin={{horizontal: 'right', vertical: 'top'}}
                  >
                    {this.props.instances.map(ins => <InstanceItem key={ins.id} {...ins} />)}
                    <Divider />
                    <MenuItem
                      style={{lineHeight: '24px', fontSize: 13}}
                      primaryText="Manage instances" onClick={this.handleManageInstances} />
                  </IconMenu>
                }
              </div>
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
