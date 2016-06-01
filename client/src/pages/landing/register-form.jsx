import React from 'react';
import ReactDOM from 'react-dom';
import { RaisedButton, TextField } from 'material-ui';
import { grey100, lightBlue500, orange500 } from 'material-ui/styles/colors';

import { postData } from '../../common/request';
import { trackEvent, trackException, trackTiming, trackView,
  PardotSignupIFrame } from '../../common/tracking';
import LoginVia from './login-via';

export default class LoginForm extends React.Component {

  constructor(props) {
    super(props);

    this.state = {
      errorText: '',
      token: null,
      email: null,
      mailSent: false,
      submitText: 'Register',
      submitting: false
    };

    this.handleKeyDown = this.handleKeyDown.bind(this);
    this._handleSubmit = this._handleSubmit.bind(this);
  }

  componentDidMount() {
    trackView('SignupForm');
  }

  handleKeyDown(ev) {
    if (ev.keyCode === 13) { // ENTER
      this._handleSubmit();
    }
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
              submitText: 'Register'
            });
            trackException(resp);
          });

        // tracking
        trackTiming('SignupButton', 'timeToClick');
        trackEvent('SignupButton', 'click');
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
        marginLeft: '2em',
        marginTop: '3px',
        verticalAlign: 'top'
      },

      confirmation: {
        textAlign: 'center',
        display: this.state.mailSent ? 'block' : 'none'
      },

      confirmationIcon: {
        fontSize: 48,
        color: lightBlue500
      },

      emailField: {
        width: 220
      },

      emailFieldLine: {
        borderColor: orange500,
        borderWidth: 2
      },

      form: {
        display: !this.state.mailSent ? 'block' : 'none',
      }
    };

    return (
      <div>
        <div style={styles.form}>
          <TextField hintText="Email" ref="emailField" type="email" errorText={this.state.errorText}
            underlineStyle={styles.emailFieldLine} style={styles.emailField}
            onKeyDown={this.handleKeyDown} />
          <RaisedButton label={this.state.submitText} style={styles.submit}
            backgroundColor={orange500} labelColor={grey100}
            disabled={this.state.submitting} onClick={this._handleSubmit} />
        </div>
        <div style={styles.confirmation}>
          <span className="fa fa-check" style={styles.confirmationIcon}></span>
          <p>A mail with further instructions was sent to {this.state.email}</p>
        </div>
        <LoginVia />
        {submitSuccess && <PardotSignupIFrame email={this.state.email} />}
      </div>
    );
  }

}
