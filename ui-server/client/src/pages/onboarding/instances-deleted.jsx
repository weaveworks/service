import React from 'react';
import { lightBlue500 } from 'material-ui/styles/colors';

import { trackView } from '../../common/tracking';
import InstancesList from '../instances/instances-list';

export default class IntancesPicker extends React.Component {

  componentDidMount() {
    trackView('InstancePicker');
  }

  render() {
    const styles = {
      confirmationIcon: {
        marginRight: 8,
        position: 'relative',
        top: 2,
        left: 2,
        fontSize: 24,
        color: lightBlue500
      },

      heading: {
        fontSize: 18,
        textTransform: 'uppercase',
        marginBottom: 18,
        marginTop: 36
      }
    };

    return (
      <div>
        <h2>
          Instance deleted
        </h2>
        <p>
          <span className="fa fa-check" style={styles.confirmationIcon}></span>
          You no longer have access to the deleted instance.
        </p>
        <div style={styles.heading}>
          Instances
        </div>
        <InstancesList />
      </div>
    );
  }

}
