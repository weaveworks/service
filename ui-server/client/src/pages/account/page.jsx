import React from 'react';
import { browserHistory } from 'react-router';

import RaisedButton from 'material-ui/RaisedButton';
import List, { ListItem } from 'material-ui/List';
import Avatar from 'material-ui/Avatar';
import FontIcon from 'material-ui/FontIcon';

import FlexContainer from '../../components/flex-container';
import Column from '../../components/column';
import Logins from './logins';
import PrivatePage from '../../components/private-page';
import { trackView, trackException } from '../../common/tracking';
import { getOrganizations } from '../../common/api';


export default class AccountPage extends React.Component {

  constructor() {
    super();
    this.state = {
      user: '',
      organizations: []
    };

    this.handleOrganizationSuccess = this.handleOrganizationSuccess.bind(this);
    this.handleOrganizationError = this.handleOrganizationError.bind(this);
  }

  componentDidMount() {
    getOrganizations()
      .then(this.handleOrganizationSuccess)
      .catch(this.handleOrganizationError);

    trackView('Account');
  }

  handleOrganizationSuccess(resp) {
    this.setState({
      user: resp.email,
      organizations: resp.organizations
    });
  }

  handleOrganizationError(resp) {
    trackException(resp);
  }

  onClickLogout() {
    browserHistory.push('/logout');
  }

  render() {
    const styles = {
      activity: {
        marginTop: 200,
        textAlign: 'center'
      },
      container: {
        marginTop: 32
      },
      avatar: {
        top: 19,
        left: 20,
        borderRadius: '3px',
      }
    };

    const logoutButton = (
      <RaisedButton
        style={{ top: 18, right: 18 }}
        secondary
        onClick={this.onClickLogout}
        label="Logout" />);

    return (
      <PrivatePage page="account" {...this.props.params}>
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
      </PrivatePage>
    );
  }

}
