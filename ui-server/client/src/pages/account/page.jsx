import React from 'react';
import { hashHistory } from 'react-router';

import RaisedButton from 'material-ui/RaisedButton';
import List, { ListItem } from 'material-ui/List';
import Avatar from 'material-ui/Avatar';
import FontIcon from 'material-ui/FontIcon';

import { FlexContainer } from '../../components/flex-container';
import { Column } from '../../components/column';
import { Logo } from '../../components/logo';
import Logins from './logins';
import Toolbar from '../../components/toolbar';
import { trackView, trackException } from '../../common/tracking';
import { getOrganizations } from '../../common/api';


export default class AccountPage extends React.Component {

  constructor() {
    super();
    this.state = {
      organizations: [],
    };

    this.checkCookie = this.checkCookie.bind(this);
    this.handleLoginSuccess = this.handleLoginSuccess.bind(this);
    this.handleLoginError = this.handleLoginError.bind(this);
  }

  componentDidMount() {
    this.checkCookie();
    trackView('Account');
  }

  checkCookie() {
    return getOrganizations().then(this.handleLoginSuccess, this.handleLoginError);
  }

  handleLoginSuccess(resp) {
    this.setState({
      user: resp.email,
      organizations: resp.organizations
    });
  }

  handleLoginError(resp) {
    if (resp.status === 401) {
      hashHistory.push('/login');
    } else if (resp.status === 403) {
      hashHistory.push('/login/forbidden');
    } else {
      const err = resp.errors[0];
      trackException(err.message);
    }
  }

  onClickLogout() {
    hashHistory.push('/logout');
  }

  render() {
    const styles = {
      activity: {
        marginTop: 200,
        textAlign: 'center'
      },
      container: {
        marginTop: 128
      },
      logoWrapper: {
        position: 'absolute',
        width: 250,
        height: 64,
        left: 64,
        top: 32 + 51 - 3
      },
      avatar: {
        top: 19,
        left: 20,
        borderRadius: '3px',
      }
    };

    const orgId = this.props.params.orgId;
    const logoutButton = (
      <RaisedButton
        style={{ top: 18, right: 18 }}
        secondary
        onClick={this.onClickLogout}
        label="Logout" />);

    return (
      <div style={{height: '100%', position: 'relative'}}>
        <Toolbar
          user={this.state.user}
          organization={orgId}
          page="Account" />
        <div style={styles.logoWrapper}>
          <Logo />
        </div>
        <div style={styles.container}>
          <FlexContainer>
            <Column minWidth="400">
              <h2>User</h2>

              {this.state.user && <List style={{ maxWidth: '32em' }}>
                <ListItem disabled
                  style={{cursor: 'default'}}
                  primaryText={this.state.user}
                  innerDivStyle={{paddingTop: 26}}
                  leftAvatar={<Avatar style={styles.avatar}
                    icon={<FontIcon className="fa fa-user" style={{left: 2}} />}
                    size={32}
                    />}
                  rightIconButton={logoutButton}
                />
              </List>}

              <Logins />
            </Column>
          </FlexContainer>
        </div>
      </div>
    );
  }

}
