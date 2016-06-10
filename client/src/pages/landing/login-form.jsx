import React from 'react';
import ReactDOM from 'react-dom';
import RaisedButton from 'material-ui/RaisedButton';
import TextField from 'material-ui/TextField';
import { amber900, grey100, grey500,
  lightBlue500 } from 'material-ui/styles/colors';
import { hashHistory } from 'react-router';

import { postData } from '../../common/request';
import { trackEvent, trackException, trackTiming, PardotSignupIFrame } from '../../common/tracking';
import LoginVia from './login-via';

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

    this.handleKeyDown = this.handleKeyDown.bind(this);
    this._handleSubmit = this._handleSubmit.bind(this);
    this._doLogin = this._doLogin.bind(this);
  }

  handleKeyDown(ev) {
    if (ev.keyCode === 13) { // ENTER
      this._handleSubmit();
    }
  }

  _doLogin() {
    const loginUrl = `/login/${this.state.email}/${this.state.token}`;
    hashHistory.push(loginUrl);
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
    const submitSuccess = Boolean(this.state.token) || this.state.mailSent;
    const unauthorized = this.props.params.error === 'unauthorized';
    const styles = {
      submit: {
        marginLeft: '2em',
        marginTop: '3px',
        verticalAlign: 'top'
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
        color: lightBlue500
      },

      confirmationLabel: {
        paddingLeft: 36
      },

      emailField: {
        width: 220
      },

      emailFieldLine: {
        borderColor: grey500,
        borderWidth: 2
      },

      emailFieldFocusLine: {
        borderColor: grey500
      },

      emailFieldHint: {
        bottom: 10
      },

      form: {
        display: !submitSuccess ? 'block' : 'none',
        textAlign: 'center'
      },

      formHint: {
        marginTop: '0.5em',
        textAlign: 'center',
        fontSize: '0.8rem'
      },

      heading: {
        fontSize: 18,
        textTransform: 'uppercase',
        marginBottom: 36
      },

      loginVia: {
        display: !submitSuccess ? 'block' : 'none',
        textAlign: 'center'
      },

      splitter: {
        display: !submitSuccess ? 'block' : 'none',
        textAlign: 'center',
        padding: '36px 0px',
        textTransform: 'uppercase'
      },

      devLink: {
        display: this.state.token ? 'block' : 'none',
        fontSize: '85%',
        opacity: 0.6
      },

      unauthorized: {
        display: 'inline-block',
        position: 'relative',
        width: 228,
        fontSize: 14,
      },

      unauthorizedIcon: {
        position: 'absolute',
        top: 0,
        left: -2,
        fontSize: 32,
        color: amber900
      },

      unauthorizedLabel: {
        color: amber900,
        paddingLeft: 32
      },

      unauthorizedWrapper: {
        marginTop: 16,
        textAlign: 'center',
        display: unauthorized ? 'block' : 'none'
      }
    };

    return (
      <div style={styles.wrapper}>
        <div style={styles.heading}>
          Log in
        </div>
        <div style={styles.loginVia}>
          <LoginVia prefix="Log in" />
        </div>
        <div style={styles.splitter}>
          or
        </div>
        <div style={styles.form}>
          <TextField hintText="Log in with your Email" ref="emailField" type="email"
            disabled={submitSuccess || this.state.submitting}
            errorText={this.state.errorText}
            underlineFocusStyle={styles.emailFieldFocusLine}
            underlineStyle={styles.emailFieldLine} style={styles.emailField}
            onKeyDown={this.handleKeyDown} />
          <RaisedButton label={this.state.submitText} style={styles.submit}
            backgroundColor={grey500} labelColor={grey100}
            disabled={this.state.submitting} onClick={this._handleSubmit} />
          <div style={styles.formHint}>
            You’ll get an email with a login token.
          </div>
        </div>
        <div style={styles.unauthorizedWrapper}>
          <div style={styles.unauthorized}>
            <span className="fa fa-ban" style={styles.unauthorizedIcon}></span>
            <div style={styles.unauthorizedLabel}>
              The login link is no longer valid. Enter your email to log in again.
            </div>
          </div>
        </div>
        <div style={styles.confirmation}>
          <span className="fa fa-check" style={styles.confirmationIcon}></span>
          <div style={styles.confirmationLabel}>
            A mail with the next step was sent to {this.state.email}
          </div>
        </div>
        <div style={styles.devLink}>
          <button onClick={this._doLogin}>Developer login link</button>
        </div>
        {submitSuccess && <PardotSignupIFrame email={this.state.email} />}
      </div>
    );
  }

}
