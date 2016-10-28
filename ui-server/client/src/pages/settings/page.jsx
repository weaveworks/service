/* eslint react/jsx-no-bind:0 */
import React from 'react';
import { Column } from '../../components/column';
import { FlexContainer } from '../../components/flex-container';

import PrivatePage from '../../components/private-page';
import Menu from '../../components/menu';
import MenuSection from '../../components/menu/section';
import MenuLink from '../../components/menu/link';
import { trackView } from '../../common/tracking';
import { encodeURIs } from '../../common/request';

export default class SettingsPage extends React.Component {
  componentDidMount() {
    // track internal view state
    trackView('Settings');
  }

  render() {
    const styles = {
      container: {
        marginTop: 32
      },
      flexContainer: {
        justifyContent: 'center',
      }
    };

    return (
      <PrivatePage page="settings" {...this.props.params}>
        <div style={styles.container}>
          <FlexContainer style={styles.flexContainer}>
            <Column width="224">
              <Menu>
                <MenuSection title="Settings">
                  <MenuLink to={encodeURIs`/settings/${this.props.params.orgId}/account`}>
                    Account
                  </MenuLink>
                </MenuSection>
                <MenuSection title="Billing">
                  <MenuLink to={encodeURIs`/settings/billing/${this.props.params.orgId}/usage`} >
                    Usage
                  </MenuLink>
                  <MenuLink to={encodeURIs`/settings/billing/${this.props.params.orgId}/invoices`} >
                    Invoices
                  </MenuLink>
                  <MenuLink to={encodeURIs`/settings/billing/${this.props.params.orgId}/register`} >
                    Register
                  </MenuLink>
                </MenuSection>
              </Menu>
            </Column>
            <Column width="700">
              {this.props.children}
            </Column>
          </FlexContainer>
        </div>
      </PrivatePage>
    );
  }
}
