import React from 'react';
import ReactDOM from 'react-dom';
import RaisedButton from 'material-ui/RaisedButton';
import TextField from 'material-ui/TextField';
import { grey100, grey500, lightBlue500 } from 'material-ui/styles/colors';
import { browserHistory } from 'react-router';

import { getLogins } from '../../common/api';
import { postData } from '../../common/request';
import { trackEvent, trackException, trackTiming, trackView,
  PardotSignupIFrame } from '../../common/tracking';
import ErrorMessage from '../../components/error-message';
import LoginVia from './login-via';

const ERROR_MESSAGES = {
  forbidden: 'Your login is no longer valid. Try logging in again.',
  unauthorized: 'The login link is no longer valid. Enter your email to log in again.'
};

export default class Form extends React.Component {

  constructor(props) {
    super(props);

    this.state = {
      email: null,
      errorText: '',
      logins: [],
      mailSent: false,
      submitText: 'Go',
      submitting: false,
      token: null
    };

    this.handleKeyDown = this.handleKeyDown.bind(this);
    this._handleSubmit = this._handleSubmit.bind(this);
    this._handleLoadAuthsSuccess = this._handleLoadAuthsSuccess.bind(this);
    this._handleLoadAuthsError = this._handleLoadAuthsError.bind(this);
    this._doLogin = this._doLogin.bind(this);
    this.getConfirmation = this.getConfirmation.bind(this);
  }

  _loadAuths() {
    getLogins().then(this._handleLoadAuthsSuccess, this._handleLoadAuthsError);
  }

  _handleLoadAuthsSuccess(resp) {
    this.setState({ logins: resp.logins });
  }

  _handleLoadAuthsError(resp) {
    trackException(resp);
    browserHistory.push('/');
  }

  _doLogin() {
    // for dev login button
    const loginUrl = `/login/${this.state.email}/${this.state.token}`;
    browserHistory.push(loginUrl);
  }

  componentDidMount() {
    this._loadAuths();
    if (this.props.formId) {
      trackView(this.props.formId);
    }
  }

  handleKeyDown(ev) {
    if (ev.keyCode === 13) { // ENTER
      this._handleSubmit();
    }
  }

  getConfirmation() {
    if (this.props.getConfirmation) {
      return this.props.getConfirmation(this.state.email);
    }
    return `A mail with the next step was sent to ${this.state.email}`;
  }

  _handleSubmit() {
    const wrapperNode = ReactDOM.findDOMNode(this.refs.emailField);
    const inputNode = wrapperNode.getElementsByTagName('input')[0];
    const value = inputNode.value;

    if (value) {
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
            // empty field
            inputNode.value = '';

            this.setState({
              mailSent: resp.mailSent,
              email: resp.email,
              token: resp.token,
              submitText: 'Done',
              submitting: false
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
        if (this.props.buttonId) {
          trackTiming(this.props.buttonId, 'timeToClick');
          trackEvent(this.props.buttonId, 'click');
        }
      } else {
        this.setState({
          errorText: 'Please provide a valid email address.'
        });
      }
    }
  }

  render() {
    const submitSuccess = Boolean(this.state.token) || this.state.mailSent;
    const errorMessage = ERROR_MESSAGES[this.props.error];
    const styles = {
      submit: {
        marginLeft: '2em',
        marginTop: '3px',
        verticalAlign: 'top'
      },

      confirmation: {
        position: 'relative',
        width: 300,
        fontSize: 14,
        marginTop: 8,
        display: submitSuccess ? 'block' : 'none'
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

      devLink: {
        display: this.state.token ? 'block' : 'none',
        fontSize: '85%',
        opacity: 0.6
      },

      splitter: {
        display: !submitSuccess ? 'block' : 'none',
        textAlign: 'center',
        padding: '36px 0px',
        textTransform: 'uppercase'
      }
    };

    const hintText = `${this.props.prefix} Email`;

    return (
      <div>
        <div style={styles.heading}>
          {this.props.title}
        </div>
        <div style={styles.loginVia}>
          <LoginVia prefix={this.props.prefix} logins={this.state.logins} />
        </div>
        <div style={styles.splitter}>
          or
        </div>
        <div style={styles.form}>
          <TextField hintText={hintText} ref="emailField" type="email"
            errorText={this.state.errorText}
            underlineFocusStyle={styles.emailFieldFocusLine}
            underlineStyle={styles.emailFieldLine} style={styles.emailField}
            onKeyDown={this.handleKeyDown} />
          <RaisedButton label={this.state.submitText} style={styles.submit}
            backgroundColor={grey500} labelColor={grey100}
            disabled={this.state.submitting} onClick={this._handleSubmit} />
          <div style={styles.formHint}>
            {this.props.children}
          </div>
        </div>

        <ErrorMessage message={errorMessage} hidden={submitSuccess} />

        <div style={styles.confirmation}>
          <span className="fa fa-check" style={styles.confirmationIcon}></span>
          <p style={styles.confirmationLabel}>
            {this.getConfirmation()}.
          </p>
        </div>
        <div style={styles.devLink}>
          <button onClick={this._doLogin}>Developer login link</button>
        </div>
        {submitSuccess && <PardotSignupIFrame email={this.state.email} />}
      </div>
    );
  }

}
