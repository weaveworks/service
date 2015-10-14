import React from "react";
import { HashLocation, RouteHandler } from "react-router";
import { getData } from "../../common/request";
import { CircularProgress, Styles } from "material-ui";

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
      featureHeader: {
        fontSize: 48,
        fontWeight: 300
      },
      featureWrapper: {
        marginRight: 48,
        marginTop: 48,
        padding: 16,
        width: 500
      },
      formContainer: {
        margin: '0 16px 16px',
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
                <em>Participation in the Early Access program is free of charge.</em>
              </div>
              <div style={styles.infoHeader}>
                Learn
              </div>
              <div style={styles.infoItem}>
                Learn more about Weave Scope <a href="http://weave.works/scope" target="website">on our website.</a>
              </div>
              <div style={styles.infoItem}>
                Build and deploy a Docker app on Amazon ECS - check out 
                  our <br /><a target="ecsguide" href="http://weave.works/guides/service-discovery-with-weave-aws-ecs.html">getting started guide</a>
              </div>
            </div>
          </div>
        </div>
      </BackgroundContainer>
    );
  }
}
