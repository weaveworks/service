import React from 'react';

import Form from './form';

export default class LoginForm extends React.Component {
  render() {
    return (
      <Form title="Log in" prefix="Log in with" buttonId="LoginButton"
        formId="LoginForm" error={this.props.params.error}>
        You’ll get an email with a login link.
      </Form>
    );
  }

}
