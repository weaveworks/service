import React from 'react';

import { FlexContainer } from '../../components/flex-container';
import { Column } from '../../components/column';
import InstancesList from '../instances/instances-list';
import PrivatePage from '../../components/private-page';
import { trackView } from '../../common/tracking';

export default class InstancesPage extends React.Component {

  componentDidMount() {
    trackView('Instances');
  }

  render() {
    const styles = {
      activity: {
        marginTop: 200,
        textAlign: 'center'
      },
      container: {
        marginTop: 32
      },
      logoWrapper: {
        position: 'absolute',
        width: 250,
        height: 64,
        left: 64,
        top: 32 + 51 - 3
      }
    };

    const orgId = this.props.params.orgId;

    return (
      <PrivatePage page="instance" {...this.props.params}>
        <div style={styles.container}>
          <FlexContainer>
            <Column>
              <h2>Your Instances</h2>
              <p>This is a list of all Weave Cloud instances you have access to:</p>
              <InstancesList currentInstance={orgId} />
            </Column>
            <Column />
          </FlexContainer>
        </div>
      </PrivatePage>
    );
  }

}
