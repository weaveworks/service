import React from 'react';
import Divider from 'material-ui/Divider';
import FlatButton from 'material-ui/FlatButton';
import FontIcon from 'material-ui/FontIcon';
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
    this.handleClickAccount = this.handleClickAccount.bind(this);
    this.handleClickInstance = this.handleClickInstance.bind(this);
    this.handleClickSettings = this.handleClickSettings.bind(this);
    this.handleManageInstances = this.handleManageInstances.bind(this);
  }

  handleClickInstance() {
    const url = encodeURIs`/app/${this.props.orgId}`;
    hashHistory.push(url);
  }

  handleClickSettings() {
    const url = encodeURIs`/org/${this.props.orgId}`;
    hashHistory.push(url);
  }

  handleClickAccount() {
    const url = encodeURIs`/account/${this.props.orgId}`;
    hashHistory.push(url);
  }

  handleManageInstances() {
    const url = encodeURIs`/instance/${this.props.orgId}`;
    hashHistory.push(url);
  }

  isActive(page) {
    const url = encodeURIs`/${page}/${this.props.orgId}`;
    return this.context.router.isActive(url);
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
      toolbarButton: {
        minWidth: 48
      },
      toolbarButtonIcon: {
        fontSize: '1.2rem',
        top: 2
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
        padding: 8
      },
      toolbarWrapper: {
        backgroundColor: '#e4e4ed',
        position: 'absolute',
        width: '100%'
      }
    };

    const viewText = this.props.instanceName ? `View ${this.props.instanceName}` : 'Loading...';
    const viewColor = this.isActive('app') ? Colors.text : Colors.text3;
    const settingsColor = this.isActive('org') ? Colors.text : Colors.text3;
    const accountColor = this.isActive('account') ? Colors.text : Colors.text3;
    const viewSelectorButton = (
      <FlatButton style={styles.toolbarButton}>
        <FontIcon className="fa fa-caret-down" color={Colors.text2}
          style={styles.toolbarButtonIcon} />
      </FlatButton>
    );

    return (
      <div>
        <Paper zDepth={1} style={styles.toolbarWrapper}>
          <div style={styles.toolbar}>
            <div style={styles.toolbarLeft}>
              <Logo />
            </div>
            <div style={styles.toolbarCenter}>
              <div style={{position: 'relative'}}>
                {this.props.instances && this.props.instances.length > 0 && <IconMenu
                  iconButtonElement={viewSelectorButton}
                  anchorOrigin={{horizontal: 'left', vertical: 'top'}}
                  targetOrigin={{horizontal: 'left', vertical: 'top'}}
                  >
                    {this.props.instances.map(ins => <InstanceItem key={ins.id} {...ins} />)}
                    <Divider />
                    <MenuItem
                      style={{lineHeight: '24px', fontSize: 13}}
                      primaryText="Manage instances" onClick={this.handleManageInstances} />
                  </IconMenu>
                }
                <FlatButton
                  style={{color: viewColor}}
                  onClick={this.handleClickInstance}
                  label={viewText} />
                <FlatButton style={styles.toolbarButton} onClick={this.handleClickSettings}>
                  <FontIcon className="fa fa-cog" color={settingsColor}
                    style={styles.toolbarButtonIcon} />
                </FlatButton>
              </div>
            </div>
            <div style={styles.toolbarRight}>
            <FlatButton style={styles.toolbarButton} labelStyle={{color: accountColor}}
              onClick={this.handleClickAccount} label="Account" />
            </div>
          </div>
        </Paper>
        <div style={styles.filler} />
      </div>
    );
  }
}

Toolbar.contextTypes = { router: React.PropTypes.object.isRequired };
