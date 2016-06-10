import React from 'react';

import { FlexContainer } from '../../components/flex-container';
import { Column } from '../../components/column';
import { Logo } from '../../components/logo';
import Logins from './logins';
import Toolbar from '../../components/toolbar';
import { trackView } from '../../common/tracking';

export default class AccountPage extends React.Component {

  constructor() {
    super();
    this.state = {};
  }

  componentDidMount() {
    trackView('Account');
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

    return (
      <div style={{height: '100%', overflowY: 'scroll', position: 'relative'}}>
        <Toolbar user={this.state.user} page="Account" />
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
