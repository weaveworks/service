import React from "react";
import { HashLocation, RouteHandler } from "react-router";
import { CircularProgress, Styles } from "material-ui";
import CookieBanner from 'react-cookie-banner';

import { getData } from "../../common/request";
import { BackgroundContainer } from "../../components/background-container";
import { Logo } from "../../components/logo";

const Colors = Styles.Colors;
const ThemeManager = new Styles.ThemeManager();

export default class LandingPage extends React.Component {

  static childContextTypes = {
    muiTheme: React.PropTypes.object
  }

  getChildContext() {
    return {
      muiTheme: ThemeManager.getCurrentTheme()
    };
  }

  componentWillMount() {
    ThemeManager.setPalette({
      accent1Color: Colors.deepOrange500
    });
  }

  renderLinks(linkStyle) {
    const links = [
      {href: 'http://weave.works', text: 'Weaveworks'},
      {href: 'http://blog.weave.works', text: 'Blog'},
      {href: 'http://weave.works/help', text: 'Support'},
    ];

    return links.map(link => {
      return (
        <a style={linkStyle} href={link.href} target="_blank">
          {link.text}
        </a>
      );
    });
  }

  render() {
    const styles = {
      container: {
        display: 'flex',
        flexDirection: 'row',
        flexWrap: 'wrap',
        justifyContent: 'center',
        alignContent: 'flex-start',
        alignItems: 'flex-start',
      },
      cookieBanner: {
        banner: {
          backgroundColor: 'rgba(50,50,75,0.7)'
        },
        message: {
          fontSize: '1rem',
          fontWeight: 300
        }
      },
      featureHeader: {
        fontSize: 48,
        fontWeight: 300
      },
      featureWrapper: {
        padding: '48px 64px 16px 16px',
        width: 500
      },
      formContainer: {
        margin: 16,
        padding: 48,
        width: 420,
        backgroundColor: 'rgba(250,250,252,0.7)',
        borderRadius: 4
      },
      formWrapper: {
        padding: 8,
        margin: '24px 8px',
      },
      infoHeader: {
        fontSize: 18,
        fontWeight: 300
      },
      infoItem: {
        fontSize: '0.8rem',
        marginTop: '0.5rem'
      },
      infoWrapper: {
      },
      headerContainer: {
        display: 'flex',
        flexDirection: 'row',
        flexWrap: 'wrap',
        justifyContent: 'space-between',
        alignContent: 'flex-start',
        alignItems: 'flex-start',
      },
      link: {
        textTransform: 'uppercase',
        marginRight: '2rem'
      },
      logoWrapper: {
        width: 250,
        height: 64,
        marginLeft: 64,
        marginTop: 32 + 51 - 3
      },
      menuWrapper: {
        padding: 64,
        marginTop: 32 - 3
      }
    };

    const links = this.renderLinks(styles.link);

    return (
      <BackgroundContainer imageUrl="landing.jpg">
        <CookieBanner message="We use two cookies, one that makes sure you stay logged in, and one that tracks usage anonymously."
          cookie="eu-user-has-accepted-cookies" buttonMessage="I'm OK with this" styles={styles.cookieBanner} />
        <div style={styles.headerContainer}>
          <div style={styles.logoWrapper}>
            <Logo />
          </div>
          <div style={styles.menuWrapper}>
            {links}
          </div>
        </div>
        <div style={styles.container}>
          <div style={styles.featureWrapper}>
            <div style={styles.featureHeader}>
              Weave Scope is the easiest way to manage and monitor your Docker Containers on AWS ECS
            </div>
          </div>
          <div style={styles.formContainer}>
            <div style={styles.infoWrapper}>
              <div style={styles.infoHeader}>
                Request an invite to our Early Access program
              </div>
            </div>

            <div style={styles.formWrapper}>
              <RouteHandler {...this.props} />
            </div>

            <div style={styles.infoWrapper}>
              <div style={styles.infoHeader}>
                How it works
              </div>
              <ol>
                <li style={styles.infoItem}>Submit your email to apply for participation in the Early Access program</li>
                <li style={styles.infoItem}>You'll receive an email with sign up details when we approve your participation.</li>
                <li style={styles.infoItem}>Follow the simple instructions in the email to install the drop-in probe container and connect it to Scope.</li>
              </ol>
              <div style={styles.infoItem}>
                <b>There is no charge for the private beta.</b>
              </div>
            </div>
          </div>
        </div>
      </BackgroundContainer>
    );
  }
}
