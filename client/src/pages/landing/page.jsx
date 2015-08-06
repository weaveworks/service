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
      errorText: '',
      loginToken: null,
      mailRecipient: null,
      mailSent: false,
      submitText: 'Go',
      submitting: false
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
    const styles = {
      submit: {
        marginLeft: '2em',
        marginTop: '3px',
        verticalAlign: 'top'
      },

      container: {
        textAlign: 'center',
        paddingTop: '200px'
      },

      confirmation: {
        display: this.state.mailSent ? "block" : "none",
        fontSize: '85%',
        opacity: 0.6
      },

      form: {
        display: !this.state.mailSent && !this.state.loginToken ? "block" : "none",
      },

      link: {
        display: this.state.loginToken ? "block" : "none",
        fontSize: '85%',
        opacity: 0.6
      }
    };

    const loginUrl = `/api/users/login?token=${this.state.loginToken}&email=${this.state.mailRecipient}`;

    return (
      <div id="landing-page" style={styles.container}>
        <h1>Scope as a Service</h1>
        <div style={styles.form}>
          <TextField hintText="Email" ref="emailField" type="email" errorText={this.state.errorText}
            onEnterKeyDown={this._handleSubmit.bind(this)} />
          <RaisedButton label={this.state.submitText} primary={true} style={styles.submit}
            disabled={this.state.submitting} onClick={this._handleSubmit.bind(this)} />
        </div>
        <div style={styles.confirmation}>
          <p>A mail with login details was sent to {this.state.mailRecipient}.</p>
        </div>
        <div style={styles.link}>
          <a href={loginUrl}>Developer login link</a>
        </div>
      </div>
    );
  }

  _handleSubmit() {
    const field = this.refs.emailField;
    const value = field.getValue();

    if (value) {
      const wrapperNode = this.refs.emailField.getDOMNode();
      const inputNode = wrapperNode.getElementsByTagName('input')[0];
      const valid = inputNode.validity.valid;

      if(valid) {
        // lock button and clear error
        this.setState({
          errorText: '',
          submitting: true,
          submitText: 'Sending...'
        });

        postData('/api/users/signup', {email: value})
          .then(function(resp) {
            // empty field
            field.clearValue();

            this.setState({
              mailSent: resp.mailSent,
              mailRecipient: resp.email,
              loginToken: resp.token
            });
          }.bind(this), function(resp) {
            this.setState({
              errorText: resp,
              submitting: false,
              submitText: 'Go'
            });
          }.bind(this));
      } else {
        this.setState({
          errorText: 'Please provide a valid email address.'
        });
      }

    }

  }

}
