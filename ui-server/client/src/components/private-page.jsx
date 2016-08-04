import React from 'react';

import { getInstance } from '../common/api';
import { trackException } from '../common/tracking';
import Toolbar from './toolbar';

export default class PrivatePage extends React.Component {

  constructor() {
    super();
    this.state = {
      email: '',
      instance: null,
      organizations: []
    };

    this.handleInstanceSuccess = this.handleInstanceSuccess.bind(this);
    this.handleInstanceError = this.handleInstanceError.bind(this);
  }

  handleInstanceSuccess(userData) {
    this.setState(userData);
  }

  handleInstanceError(res) {
    trackException(res);
  }

  componentDidMount() {
    if (this.props.orgId) {
      // includes a cookie check
      getInstance(this.props.orgId)
        .then(this.handleInstanceSuccess)
        .catch(this.handleInstanceError);
    }
  }

  render() {
    const styles = {
      backgroundContainer: {
        height: '100%',
        position: 'relative'
      }
    };

    return (
      <div style={styles.backgroundContainer}>
        <Toolbar
          page={this.props.page}
          instances={this.state.organizations}
          instance={this.state.instance}
          user={this.state.email}
          orgId={this.props.orgId} />
        {this.props.children}
      </div>
    );
  }
}
