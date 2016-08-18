/* eslint react/jsx-no-bind:0 */
import React from 'react';

import { encodeURIs } from '../../common/request';
import PrivatePage from '../../components/private-page';
import { trackView } from '../../common/tracking';

export default class PromWrapperPage extends React.Component {

  componentDidMount() {
    trackView('Prom');
  }

  render() {
    const styles = {
      iframe: {
        display: 'block',
        border: 'none',
        height: 'calc(100vh - 56px)',
        width: '100%'
      }
    };

    const { orgId } = this.props.params;
    const frameUrl = encodeURIs`/api/app/${orgId}/api/prom/graph`;

    return (
      <PrivatePage page="prom" {...this.props.params}>
        <iframe src={frameUrl} style={styles.iframe} />
      </PrivatePage>
    );
  }
}
