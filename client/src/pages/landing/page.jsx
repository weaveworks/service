import React from "react";
import { postData } from "../../common/request";
import { Styles, RaisedButton, TextField } from "material-ui";

const Colors = Styles.Colors;
const ThemeManager = new Styles.ThemeManager();

export default class LandingPage extends React.Component {

  static childContextTypes = {
    muiTheme: React.PropTypes.object
  }

  constructor(props) {
    super(props);
    this.state = {
      mailRecipient: null,
      mailSent: false,
      loginToken: null
    };
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

  render() {
    let buttonStyle = {
      marginLeft: '2em'
    };

    let containerStyle = {
      textAlign: 'center',
      paddingTop: '200px'
    };

    let confirmationStyle = {
      display: this.state.mailSent ? "block" : "none",
      fontSize: '85%',
      opacity: 0.6
    };

    let linkStyle = {
      display: this.state.loginToken ? "block" : "none",
      fontSize: '85%',
      opacity: 0.6
    };

    let loginUrl = `/api/users/login?token=${this.state.loginToken}&email=${this.state.mailRecipient}`;

    return (
      <div id="landing-page" style={containerStyle}>
        <h1>Scope as a Service</h1>
        <TextField hintText="Email" ref="emailField" />
        <RaisedButton label="Go" primary={true} style={buttonStyle} onClick={this._handleTouchTap.bind(this)} />
        <div style={confirmationStyle}>
          <p>A mail with login details was sent to {this.state.mailRecipient}.</p>
        </div>
        <div style={linkStyle}>
          <a href={loginUrl}>Secret login link</a>
        </div>
      </div>
    );
  }

  _handleTouchTap() {
    // TODO trigger sending of email

    // TODO show "Sending email"

    const email = this.refs.emailField.getValue();

    postData('/api/users/signup', {email: email})
      .then(function(resp) {
        this.setState({
          mailSent: resp.mailSent,
          mailRecipient: resp.email,
          loginToken: resp.token
        });
      }.bind(this), function(resp) {
        // TODO show error
        console.error(resp);
      }.bind(this));
  }

}
