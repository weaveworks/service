import React from 'react';
import { hashHistory } from 'react-router';

import { FlexContainer } from '../../components/flex-container';
import PublicPage from '../../components/public-page';
import { trackException } from '../../common/tracking';
import { getOrganizations } from '../../common/api';

export default class OnboardingPage extends React.Component {

  constructor() {
    super();

    this.handleCookieSuccess = this.handleCookieSuccess.bind(this);
    this.handleCookieError = this.handleCookieError.bind(this);
  }

  componentDidMount() {
    this.checkCookie();
  }

  checkCookie() {
    return getOrganizations().then(this.handleCookieSuccess, this.handleCookieError);
  }

  handleCookieSuccess() {
    // noop, just checking logged-in state
  }

  handleCookieError(resp) {
    if (resp.status !== 401) {
      const err = resp.errors[0];
      trackException(err.message);
    }
    hashHistory.push('/signup');
  }

  render() {
    return (
      <PublicPage>
        <FlexContainer style={{alignItems: 'center'}}>
          {this.props.children}
        </FlexContainer>
      </PublicPage>
    );
  }

}
