import React from "react";
import { HashLocation } from "react-router";
import { getData, postData } from "../../common/request";
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
      token: null,
      email: null,
      mailSent: false,
      submitText: 'Go',
      submitting: false
    };

    this._handleSubmit = this._handleSubmit.bind(this);
    this._handleLoginSuccess = this._handleLoginSuccess.bind(this);
    this._handleLoginError = this._handleLoginError.bind(this);
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

    // triggered on fresh page load with login params
    this._tryLogin();
  }

  componentDidUpdate(prevProps, prevState) {
    // triggered via URL hashchange
    if (!this.state.errorText && !this.state.submitting) {
      this._tryLogin();
    }
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
        display: !this.state.mailSent && !this.state.token ? "block" : "none",
      },

      link: {
        display: this.state.token ? "block" : "none",
        fontSize: '85%',
        opacity: 0.6
      }
    };

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
          <p>A mail with login details was sent to {this.state.email}.</p>
        </div>
        <div style={styles.link}>
          <button onClick={this._doLogin.bind(this)}>Developer login link</button>
        </div>
      </div>
    );
  }

  _tryLogin() {
    const params = this.props.params;
    const email = this.state.email ? this.state.email : params.email;
    const token = this.state.token ? this.state.token : params.token;
    if (email && token) {
      const url = `/api/users/login?email=${email}&token=${token}`;
      getData(url).then(this._handleLoginSuccess, this._handleLoginError);
    }
  }

  _doLogin() {
    const loginUrl = `#/login/${this.state.email}/${this.state.token}`;
    HashLocation.push(loginUrl);
  }

  _handleLoginSuccess(resp) {
    const url = `/org/${resp.organizationName}`
    HashLocation.push(url);
  }

  _handleLoginError(resp) {
    this.setState({
      errorText: resp.errors[0].message
    });
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
              email: resp.email,
              token: resp.token,
              submitting: false
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
