import React from "react";
import { HashLocation, RouteHandler } from "react-router";
import { getData } from "../../common/request";
import { CircularProgress, Styles } from "material-ui";

import { Logo } from "../../components/logo";

const Colors = Styles.Colors;
const ThemeManager = new Styles.ThemeManager();

export default class LandingPage extends React.Component {

  constructor() {
    super();

    this.state = {
      shiftX: 50,
      shiftY: 60
    };
  }

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
      {href: 'http://weave.works', text: 'Weave'},
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
    let backgroundPosition = `${this.state.shiftX}% ${this.state.shiftY}%`;
    const styles = {
      background: {
        backgroundImage: 'url("landing.jpg")',
        backgroundPosition: backgroundPosition,
        backgroundRepeat: 'no-repeat',
        position: 'fixed',
        top: 0,
        bottom: 0,
        left: 0,
        right: 0,
        zIndex: -10
      },
      container: {
        height: '100%',
        paddingTop: 200,
        display: 'flex',
        flexDirection: 'row',
        flexWrap: 'wrap',
        justifyContent: 'center',
        alignContent: 'flex-start',
        alignItems: 'flex-start'
      },
      featureHeader: {
        fontSize: 36,
        fontWeight: 300,
        marginTop: '2rem'
      },
      featureWrapper: {
        padding: 64,
        width: 400
      },
      formContainer: {
        padding: 64,
        width: 400
      },
      formWrapper: {
        marginTop: 48,
        marginBottom: 48
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
      link: {
        textTransform: 'uppercase',
        marginRight: '2rem'
      },
      logoWrapper: {
        position: 'absolute',
        width: 250,
        height: 64,
        left: 64,
        top: 32 + 51 - 3
      },
      menuWrapper: {
        position: 'absolute',
        right: 64,
        top: 32 + 51 + 6
      }
    };

    const links = this.renderLinks(styles.link);

    return (
      <div style={{height: '100%'}} onMouseMove={this.handleMouseMove.bind(this)}>
        <div style={styles.background}></div>
        <div style={styles.container}>
          <div style={styles.logoWrapper}>
            <Logo />
          </div>
          <div style={styles.menuWrapper}>
            {links}
          </div>
          <div style={styles.featureWrapper}>
            <div style={styles.featureHeader}>
              Container Visibility
            </div>
            <div style={styles.featureText}>
              Weave Scope automatically generates a map of your containers
            </div>
            <div style={styles.featureHeader}>
              Container Monitoring
            </div>
            <div style={styles.featureText}>
              Understand, monitor, and control your applications
            </div>
          </div>
          <div style={styles.formContainer}>
            <div style={styles.infoWrapper}>
              <div style={styles.infoHeader}>
                Start monitoring your containers
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
                <li style={styles.infoItem}>Fill out the email form to apply for participation in our beta program</li>
                <li style={styles.infoItem}>Once approved, you’ll receive an email with a login link</li>
                <li style={styles.infoItem}>Follow the instructions in the email to install the drop-in probe container and how to connect it to Scope</li>
              </ol>
              <div style={styles.infoItem}>
                There is no charge for the private beta.
              </div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  handleMouseMove(e) {
    const centerX = window.innerWidth / 2;
    const centerY = window.innerHeight / 2;
    const maxShiftPercent = 5;
    const shiftX = 50 + (centerX - e.clientX) / centerX * maxShiftPercent;
    const shiftY = 60 + (centerY - e.clientY) / centerY * maxShiftPercent;

    this.setState({shiftX: shiftX, shiftY: shiftY});
  }
}
