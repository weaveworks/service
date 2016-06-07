import React from 'react';
import { hashHistory } from 'react-router';
import _ from 'lodash';

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
    this.setState({ organizations: resp.organizations });
  }

  handleLoginError(resp) {
    if (resp.status === 401) {
      hashHistory.push('/login');
    } else {
      const err = resp.errors[0];
      trackException(err.message);
    }
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
      }
    };

    const orgName = _.get(this.state, ['organizations', 0, 'name']);

    return (
      <div style={{height: '100%', overflowY: 'scroll', position: 'relative'}}>
        <Toolbar
          user={this.state.user}
          organization={orgName}
          page="Account" />
        <div style={styles.logoWrapper}>
          <Logo />
        </div>
        <div style={styles.container}>
          <FlexContainer>
            <Column minWidth="400">
              <h1>Configure your account</h1>
              <Logins />
            </Column>
          </FlexContainer>
        </div>
      </div>
    );
  }

}
