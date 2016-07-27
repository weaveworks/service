import React from 'react';
import { hashHistory } from 'react-router';

import { FlexContainer } from '../../components/flex-container';
import { Column } from '../../components/column';
import { Logo } from '../../components/logo';
import InstancesList from '../instances/instances-list';
import Toolbar from '../../components/toolbar';
import { trackView, trackException } from '../../common/tracking';
import { getOrganizations } from '../../common/api';

export default class InstancesPage extends React.Component {

  constructor() {
    super();
    this.state = {
      user: '',
      organizations: [],
    };

    this.checkCookie = this.checkCookie.bind(this);
    this.handleLoginSuccess = this.handleLoginSuccess.bind(this);
    this.handleLoginError = this.handleLoginError.bind(this);
  }

  componentDidMount() {
    this.checkCookie();
    trackView('Instances');
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

    const orgId = this.props.params.orgId;

    return (
      <div style={{height: '100%', position: 'relative', paddingBottom: 64}}>
        <Toolbar
          user={this.state.user}
          organization={orgId}
          page="Instances" />
        <div style={styles.logoWrapper}>
          <Logo />
        </div>
        <div style={styles.container}>
          <FlexContainer>
            <Column>
              <h2>Your Instances</h2>
              <p>This is a list of all Weave Cloud instances you have access to:</p>
              <InstancesList currentInstance={orgId} />
            </Column>
            <Column />
          </FlexContainer>
        </div>
      </div>
    );
  }

}
