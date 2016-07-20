import React from 'react';
import FlatButton from 'material-ui/FlatButton';
import { grey300 } from 'material-ui/styles/colors';
import { hashHistory } from 'react-router';

import { Logo } from './logo';

export default class PublicPage extends React.Component {

  handleClickLogin() {
    hashHistory.push('/login');
  }

  handleClickSignup() {
    hashHistory.push('/signup');
  }

  renderLinks(linkStyle) {
    const links = [
      {href: 'https://www.weave.works/guides/using-weave-scope-cloud-service-to-visualize-and-monitor-docker-containers/', text: 'Learn more'},
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
      headerContainer: {
        display: 'flex',
        flexDirection: 'row',
        flexWrap: 'wrap',
        justifyContent: 'right',
        marginBottom: 36,
        marginTop: 36,
        marginRight: 24
      },
      link: {
        textTransform: 'uppercase',
        padding: '12px 1rem'
      },
      loginWrapper: {
        padding: '26px 24px 16px 24px'
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
          {this.props.showLogin && <div style={styles.loginWrapper}>
            <FlatButton backgroundColor={grey300} label="Log in"
              onClick={this.handleClickLogin} />
          </div>}
          {this.props.showSignup && <div style={styles.loginWrapper}>
            <FlatButton backgroundColor={grey300} label="Sign up"
              onClick={this.handleClickSignup} />
          </div>}
        </div>
        {this.props.children}
      </div>
    );
  }
}
