import React from 'react';
import { grey200 } from 'material-ui/styles/colors';

import { FlexContainer } from '../../components/flex-container';
import PublicPage from '../../components/public-page';

export default class LandingPage extends React.Component {

  render() {
    const isOnSignup = this.props.location.pathname === '/signup';
    const showingLogin = isOnSignup;
    const showingSignup = this.props.location.pathname === '/login';
    const styles = {
      featureHeader: {
        fontSize: 48,
        fontWeight: 300
      },
      featureWrapper: {
        display: isOnSignup ? 'block' : 'none',
        padding: 16,
        width: 500
      },
      formContainer: {
        marginBottom: 16,
        padding: '12px 24px 24px',
        width: 480,
        backgroundColor: grey200,
        border: '1px solid #2DB5CA',
        borderRadius: 6
      },
      formWrapper: {
        padding: 12,
        margin: '12px 8px',
      },
      infoHeader: {
        marginTop: 32,
        fontSize: 18,
        fontWeight: 300
      },
      infoItem: {
        fontSize: '1rem',
        marginTop: '1rem'
      }
    };

    return (
      <PublicPage showLogin={showingLogin} showSignup={showingSignup}>
        <FlexContainer>
          <div style={styles.featureWrapper}>
            <div style={styles.featureHeader}>
              Weave Cloud is a fast and simple way to visualize,
              manage and monitor containers and microservices
            </div>
            <div style={styles.infoItem}>
              Want to find out more about Weaveworks and our products?
              <br /><a href="https://www.weave.works/"
                target="website">Check out our website</a>.
            </div>
          </div>
          <div style={styles.formContainer}>
            <div style={styles.formWrapper}>
              {this.props.children}
            </div>
          </div>
        </FlexContainer>
      </PublicPage>
    );
  }
}
