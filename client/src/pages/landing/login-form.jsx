import React from 'react';
import ReactDOM from 'react-dom';
import { Styles, FlatButton, TextField } from 'material-ui';

import { postData } from '../../common/request';
import { trackEvent, trackException, trackTiming, PardotSignupIFrame } from '../../common/tracking';

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

  _doLogin() {
    const loginUrl = `/login/${this.state.email}/${this.state.token}`;
    this.props.history.push(loginUrl);
  }

  _handleSubmit() {
    const field = this.refs.emailField;
    const value = field.getValue();

    if (value) {
      const wrapperNode = ReactDOM.findDOMNode(this.refs.emailField);
      const inputNode = wrapperNode.getElementsByTagName('input')[0];
      const valid = inputNode.validity.valid;

      if (valid) {
        // lock button and clear error
        this.setState({
          errorText: '',
          submitting: true,
          submitText: 'Sending...'
        });

        // disable input field
        inputNode.disabled = true;

        postData('/api/users/signup', {email: value})
          .then(resp => {
            this.setState({
              mailSent: resp.mailSent,
              email: resp.email,
              token: resp.token,
              submitting: false,
              submitText: 'Done'
            });
          }, resp => {
            this.setState({
              errorText: resp,
              submitting: false,
              submitText: 'Go'
            });
            trackException(resp);
          });

        // tracking
        trackTiming('LoginButton', 'timeToClick');
        trackEvent('LoginButton', 'click');
      } else {
        this.setState({
          errorText: 'Please provide a valid email address.'
        });
      }
    }
  }

  render() {
    const submitSuccess = this.state.token || this.state.mailSent;
    const styles = {
      submit: {
        marginLeft: '1em',
        marginTop: '11px',
        verticalAlign: 'top',
        minWidth: 35,
        lineHeight: '30px',
        fontSize: '12px'
      },

      submitLabel: {
        padding: '0 8px'
      },

      confirmation: {
        position: 'relative',
        width: 228,
        fontSize: 14,
        marginTop: 8,
        display: this.state.mailSent ? 'block' : 'none'
      },

      confirmationIcon: {
        position: 'absolute',
        top: 2,
        left: 2,
        fontSize: 24,
        color: Colors.lightBlue500
      },

      confirmationLabel: {
        paddingLeft: 36
      },

      emailField: {
        width: 180,
        fontSize: '14px'
      },

      emailFieldLine: {
        borderColor: Colors.blueGrey200,
        borderWidth: 1
      },

      emailFieldFocusLine: {
        borderColor: Colors.blueGrey400
      },

      emailFieldInput: {
        bottom: -2
      },

      emailFieldHint: {
        bottom: 10
      },

      devLink: {
        display: this.state.token ? 'block' : 'none',
        fontSize: '85%',
        opacity: 0.6
      },

      wrapper: {
      }
    };

    return (
      <div style={styles.wrapper}>
        <div style={styles.form}>
          <TextField hintText="Login with your email" ref="emailField" type="email"
            disabled={submitSuccess || this.state.submitting}
            errorText={this.state.errorText} inputStyle={styles.emailFieldInput}
            underlineStyle={styles.emailFieldLine} underlineFocusStyle={styles.emailFieldFocusLine}
            style={styles.emailField} hintStyle={styles.emailFieldHint}
            onEnterKeyDown={this._handleSubmit.bind(this)} />
          <FlatButton label={this.state.submitText} style={styles.submit}
            backgroundColor={Colors.blueGrey100} labelStyle={styles.submitLabel}
            disabled={submitSuccess || this.state.submitting} onClick={this._handleSubmit.bind(this)} />
        </div>
        <div style={styles.confirmation}>
          <span className="fa fa-check" style={styles.confirmationIcon}></span>
          <div style={styles.confirmationLabel}>
            A mail with the login link was sent to {this.state.email}
          </div>
        </div>
        <div style={styles.devLink}>
          <button onClick={this._doLogin.bind(this)}>Developer login link</button>
        </div>
        {submitSuccess && <PardotSignupIFrame email={this.state.email} />}
      </div>
    );
  }

}
