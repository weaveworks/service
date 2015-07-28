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
      mailSent: false
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

    let linkStyle = {
      display: this.state.mailSent ? "block" : "none",
      fontSize: '85%',
      opacity: 0.6
    };

    return (
      <div id="landing-page" style={containerStyle}>
        <h1>Scope as a Service</h1>
        <TextField hintText="Email" ref="emailField" />
        <RaisedButton label="Go" primary={true} style={buttonStyle} onClick={this._handleTouchTap.bind(this)} />
        <div style={linkStyle}>
          <p>A mail with login details was sent to {this.state.mailRecipient}.</p>
          <a href="login?token=">Secret login link</a>
        </div>
      </div>
    );
  }

  _handleTouchTap() {
    // TODO trigger sending of email

    // TODO show "Sending email"

    const email = this.refs.emailField.getValue();

    postData('/api/signup', {email: email})
      .then(function(resp) {
        this.setState({
          mailSent: resp.mailSent,
          mailRecipient: resp.email
        });
      }.bind(this), function(resp) {
        // TODO show error
      }.bind(this));
  }

}
