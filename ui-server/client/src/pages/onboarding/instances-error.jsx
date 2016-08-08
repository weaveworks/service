import React from 'react';

import { trackView } from '../../common/tracking';
import InstancePicker from './instances-picker';
import ErrorMessage from '../../components/error-message';

export default class IntancesPicker extends React.Component {

  componentDidMount() {
    trackView('InstancesError');
  }

  render() {
    return (
      <div>
        <InstancePicker />
        <ErrorMessage
          message="Instance not found. If it exists, ask your team members to invite you." />
      </div>
    );
  }

}
