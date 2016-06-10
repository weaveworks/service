import React from 'react';
import { FlatButton } from 'material-ui';
import { grey200, grey300 } from 'material-ui/styles/colors';

import { FlexContainer } from '../../components/flex-container';
import { Logo } from '../../components/logo';

export default class LandingPage extends React.Component {

  renderLinks(linkStyle) {
    const links = [
      {href: 'http://weave.works/help', text: 'Help'}
    ];

    return links.map(link => (
      <a style={linkStyle} href={link.href} key={link.text} target="_blank">
        {link.text}
      </a>
    ));
  }

  render() {
    const styles = {
      backgroundContainer: {
      },
      featureHeader: {
        fontSize: 48,
        fontWeight: 300
      },
      featureWrapper: {
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
      },
      infoWrapper: {
      },
      headerContainer: {
        display: 'flex',
        flexDirection: 'row',
        flexWrap: 'wrap',
        justifyContent: 'right',
        marginBottom: 36,
        marginTop: 36
      },
      link: {
        textTransform: 'uppercase',
        padding: '12px 1rem'
      },
      loginWrapper: {
        padding: '26px 48px 16px 24px'
      },
      logoWrapper: {
        width: 250,
        height: 64,
        marginLeft: 64,
        marginTop: 24
      },
      menuWrapper: {
        padding: 16,
        marginTop: 20
      },
      spaceWrapper: {
        flex: 1
      }
    };

    const links = this.renderLinks(styles.link);
    return (
      <div style={styles.backgroundContainer}>
        <div style={styles.headerContainer}>
          <div style={styles.logoWrapper}>
            <Logo />
          </div>
          <div style={styles.spaceWrapper}>
          </div>
          <div style={styles.menuWrapper}>
            {links}
          </div>
          <div style={styles.loginWrapper}>
            <FlatButton backgroundColor={grey300} label="Log in" />
          </div>
        </div>
        <FlexContainer>
          <div style={styles.featureWrapper}>
            <div style={styles.featureHeader}>
              Weave Scope is a fast and simple way to visualize,
              manage and monitor containers and microservices
            </div>

            <div style={styles.infoWrapper}>
              <div style={styles.infoItem}>
                Learn more about Weave Scope <a href="http://weave.works/scope" target="website">on our website.</a>
              </div>
              <div style={styles.infoItem}>
                Check out our <a href="https://www.weave.works/guides/"
                  target="guides">getting started guides</a>.
              </div>
            </div>
          </div>
          <div style={styles.formContainer}>
            <div style={styles.formWrapper}>
              {this.props.children}
            </div>
          </div>
        </FlexContainer>
      </div>
    );
  }
}
