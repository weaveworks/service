import React from 'react';
import { browserHistory } from 'react-router';

import Form from './form';

export default class SignupForm extends React.Component {

  handleClickLogin(ev) {
    ev.preventDefault();
    browserHistory.push('/login');
  }

  getConfirmation(email) {
    return `We just sent you a verification email with a link to ${email}`;
  }

  render() {
    return (
      <Form title="Sign up" prefix="Sign up with" buttonId="SignupButton"
        formId="SignupForm" getConfirmation={this.getConfirmation}>
        Already have an account? <a href="/login" onClick={this.handleClickLogin}>Log in</a>
      </Form>
    );
  }

}
