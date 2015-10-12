import React from "react";
import { HashLocation } from "react-router";
import { Styles, RaisedButton, TextField } from "material-ui";

import { getData, postData } from "../../common/request";
import { pageView } from '../../common/tracking';

const Colors = Styles.Colors;

export default class LoginForm extends React.Component {

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
  }

  componentDidMount() {
    pageView('SignupForm');
  }

  render() {
    const styles = {
      submit: {
        marginLeft: '2em',
        marginTop: '3px',
        verticalAlign: 'top'
      },

      confirmation: {
        textAlign: 'center',
        display: this.state.mailSent ? "block" : "none"
      },

      confirmationIcon: {
        fontSize: 48,
        color: Colors.lightBlue500
      },

      emailField: {
        width: 220
      },

      emailFieldLine: {
        borderColor: Colors.orange500,
        borderWidth: 2
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
      <div>
        <div style={styles.form}>
          <TextField hintText="Email" ref="emailField" type="email" errorText={this.state.errorText}
            underlineStyle={styles.emailFieldLine} style={styles.emailField}
            onEnterKeyDown={this._handleSubmit.bind(this)} />
          <RaisedButton label={this.state.submitText} primary={true} style={styles.submit}
            disabled={this.state.submitting} onClick={this._handleSubmit.bind(this)} />
        </div>
        <div style={styles.confirmation}>
          <span className="fa fa-check" style={styles.confirmationIcon}></span>
          <p>A mail with further instructions was sent to {this.state.email}</p>
        </div>
        <div style={styles.link}>
          <button onClick={this._doLogin.bind(this)}>Developer login link</button>
        </div>
      </div>
    );
  }

  _doLogin() {
    const loginUrl = `#/login/${this.state.email}/${this.state.token}`;
    HashLocation.push(loginUrl);
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

        // disable input field
        inputNode.disabled = true;

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

            // GA tracking
            if (window.ga) {
              window.ga('send', 'event', 'serviceSignupButton', '')
            }

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
