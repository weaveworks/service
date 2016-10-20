/* eslint react/jsx-no-bind:0 */
import React from 'react';

import IFrame from '../../../components/iframe';

export default class BillingSettings extends React.Component {
  render() {
    return <IFrame src={this.props.location.pathname.replace('/settings', '')} />;
  }
}
