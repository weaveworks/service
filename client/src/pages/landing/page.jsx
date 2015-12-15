import React from 'react';
import CookieBanner from 'react-cookie-banner';
import ThemeManager from 'material-ui/lib/styles/theme-manager';
import LightRawTheme from 'material-ui/lib/styles/raw-themes/light-raw-theme';
import Colors from 'material-ui/lib/styles/colors';

import { BackgroundContainer } from '../../components/background-container';
import { FlexContainer } from '../../components/flex-container';
import { Logo } from '../../components/logo';
import RegisterForm from './register-form';

export default class LandingPage extends React.Component {

  constructor() {
    super();

    this.state = {
      muiTheme: ThemeManager.getMuiTheme(LightRawTheme)
    };
  }

  getChildContext() {
    return {
      muiTheme: this.state.muiTheme
    };
  }

  componentWillMount() {
    const newMuiTheme = ThemeManager.modifyRawThemePalette(this.state.muiTheme, {
      accent1Color: Colors.deepOrange500,
    });

    this.setState({muiTheme: newMuiTheme});
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
        marginTop: 48,
        padding: 16,
        width: 500
      },
      formContainer: {
        marginBottom: 16,
        padding: '12px 48px 48px',
        width: 420,
        backgroundColor: 'rgba(250,250,252,0.7)',
        borderRadius: 4
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
        fontSize: '0.8rem',
        marginTop: '0.5rem'
      },
      infoWrapper: {
      },
      headerContainer: {
        display: 'flex',
        flexDirection: 'row',
        flexWrap: 'wrap',
        justifyContent: 'center',
        marginBottom: 36,
        marginTop: 56
      },
      link: {
        textTransform: 'uppercase',
        padding: '12px 1rem'
      },
      loginWrapper: {
        width: 280,
        padding: '16px 48px 16px 24px'
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
      <BackgroundContainer imageUrl="landing.jpg">
        <CookieBanner message="We use two cookies, one that makes sure you stay logged in, and one that tracks usage anonymously."
          cookie="eu-user-has-accepted-cookies" buttonMessage="I'm OK with this" styles={styles.cookieBanner} />
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
            {this.props.children}
          </div>
        </div>
        <FlexContainer>
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
              <RegisterForm />
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
                <em>Participation in the Early Access program is free of charge.</em>
              </div>
              <div style={styles.infoHeader}>
                Learn
              </div>
              <div style={styles.infoItem}>
                Learn more about Weave Scope <a href="http://weave.works/scope" target="website">on our website.</a>
              </div>
              <div style={styles.infoItem}>
                Build and deploy a Docker app on Amazon ECS&mdash;check out our
                  <br /><a target="ecsguide" href="http://weave.works/guides/service-discovery-with-weave-aws-ecs.html">getting started guide</a>
              </div>
            </div>
          </div>
        </FlexContainer>
      </BackgroundContainer>
    );
  }
}

LandingPage.childContextTypes = {
  muiTheme: React.PropTypes.object
};
