import React from 'react';
import Divider from 'material-ui/Divider';
import FlatButton from 'material-ui/FlatButton';
import FontIcon from 'material-ui/FontIcon';
import IconMenu from 'material-ui/IconMenu';
import MenuItem from 'material-ui/MenuItem';
import Paper from 'material-ui/Paper';
import { browserHistory } from 'react-router';

import { encodeURIs } from '../common/request';
import Colors from '../common/colors';

import InstanceItem from './instance-item';
import { Logo } from './logo';

export default class Toolbar extends React.Component {

  constructor(props, context) {
    super(props, context);
    this.handleClickAccount = this.handleClickAccount.bind(this);
    this.handleClickInstance = this.handleClickInstance.bind(this);
    this.handleClickProm = this.handleClickProm.bind(this);
    this.handleClickSettings = this.handleClickSettings.bind(this);
    this.handleClickCreateInstance = this.handleClickCreateInstance.bind(this);
  }

  handleClickInstance() {
    const url = encodeURIs`/app/${this.props.orgId}`;
    browserHistory.push(url);
  }

  handleClickSettings() {
    const url = encodeURIs`/org/${this.props.orgId}`;
    browserHistory.push(url);
  }

  handleClickAccount() {
    let url = encodeURIs`/account/${this.props.orgId}`;
    if (this.hasFeatureFlag('billing')) {
      url = encodeURIs`/settings/${this.props.orgId}/account`;
    }
    browserHistory.push(url);
  }

  hasFeatureFlag(flag) {
    const { instance } = this.props;
    return instance && instance.featureFlags
      && instance.featureFlags.indexOf(flag) > -1;
  }

  handleClickProm() {
    const url = encodeURIs`/prom/${this.props.orgId}`;
    browserHistory.push(url);
  }

  componentWillUnmount() {
    //
    // This is usually called after a 1ms delay by the IconMenu component, but, the way we do
    // navigation doesn't give it a chance to run (gets unmounted+clearTimeout), so we call it
    // explicitly.
    //
    this.props.instancesMenuRequestChange(false);
  }

  handleClickCreateInstance() {
    const url = encodeURIs`/instances/create`;
    browserHistory.push(url);
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
        //
        // prevent scrollbar appearance from "pushing" content into the page.
        // scrollbar instead appears to be overlayed over everything.
        //
        width: 'calc(100vw - 16px)',
      },
      toolbarButton: {
        minWidth: 48
      },
      toolbarButtonIcon: {
        fontSize: '1.2rem',
        top: 2
      },
      toolbarCenter: {
        padding: 8,
        position: 'relative',
        display: 'flex',
        // gotta set w/ overflowing things otherwise minWidth is something complicated like the
        // size of the overflowing stuff.
        minWidth: 0,
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
      },
      buttonLabelStyle: {
        // width fit the container
        display: 'block',
        // ellipsis!
        textOverflow: 'ellipsis',
        overflow: 'hidden',
        whiteSpace: 'nowrap',
      }
    };

    const { instance, instancesMenuOpen, instancesMenuRequestChange } = this.props;
    const viewText = instance ? `View ${instance.name}` : 'Loading...';
    const viewColor = this.isActive('app') ? Colors.text : Colors.text3;
    const settingsColor = this.isActive('org') ? Colors.text : Colors.text3;
    const accountColor = this.isActive('account') ? Colors.text : Colors.text3;
    const promColor = this.isActive('prom') ? Colors.text : Colors.text3;
    const hasProm = this.hasFeatureFlag('cortex');
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
              <IconMenu
                // don't animate onClose
                animated={false}
                // close immediately don't wait for 200ms.
                touchTapCloseDelay={1}
                open={instancesMenuOpen}
                onRequestChange={instancesMenuRequestChange}
                iconButtonElement={viewSelectorButton}
                anchorOrigin={{horizontal: 'left', vertical: 'top'}}
                targetOrigin={{horizontal: 'left', vertical: 'top'}}>
                  {this.props.instances.map(ins => <InstanceItem key={ins.id} {...ins} />)}
                  <Divider />
                  <MenuItem
                    style={{lineHeight: '24px', fontSize: 13, cursor: 'pointer'}}
                    primaryText="Create new instance"
                    onClick={this.handleClickCreateInstance} />
              </IconMenu>
              <FlatButton
                style={{color: viewColor}}
                title={viewText}
                labelStyle={styles.buttonLabelStyle}
                onClick={this.handleClickInstance}
                label={viewText} />
              {hasProm && <FlatButton style={styles.toolbarButton}
                onClick={this.handleClickProm}>
                  <FontIcon className="fa fa-area-chart" color={promColor}
                    style={styles.toolbarButtonIcon} />
                </FlatButton>
              }
              <FlatButton style={styles.toolbarButton} onClick={this.handleClickSettings}>
                <FontIcon className="fa fa-cog" color={settingsColor}
                  style={styles.toolbarButtonIcon} />
              </FlatButton>
            </div>
            <div style={styles.toolbarRight}>
            <FlatButton style={styles.toolbarButton} labelStyle={{color: accountColor}}
              onClick={this.handleClickAccount} label={this.props.user} />
            </div>
          </div>
        </Paper>
        <div style={styles.filler} />
      </div>
    );
  }
}

Toolbar.contextTypes = { router: React.PropTypes.object.isRequired };
