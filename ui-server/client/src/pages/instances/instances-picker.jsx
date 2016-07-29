import React from 'react';

import { trackView } from '../../common/tracking';
import InstancesList from './instances-list';

export default class IntancesPicker extends React.Component {

  componentDidMount() {
    trackView('InstancePicker');
  }

  render() {
    const styles = {
      heading: {
        fontSize: 18,
        textTransform: 'uppercase',
        marginBottom: 36
      }
    };

    return (
      <div>
        <div style={styles.heading}>
          Weave Cloud Instances
        </div>
        <p>Choose the Weave Cloud instance you want to access:</p>
        <InstancesList />
      </div>
    );
  }

}
