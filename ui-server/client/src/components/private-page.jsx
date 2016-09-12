import React from 'react';
import { connect } from 'react-redux';

import { getInstance, requestInstancesMenuChange } from '../actions';
import Toolbar from './toolbar';

class PrivatePage extends React.Component {

  componentDidMount() {
    if (this.props.orgId) {
      //
      // includes:
      // - a cookie check
      // - a 404 check
      //
      this.props.getInstance(this.props.orgId);
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
          instancesMenuOpen={this.props.instancesMenuOpen}
          instancesMenuRequestChange={this.props.requestInstancesMenuChange}
          instances={this.props.instanceList}
          instance={this.props.instance}
          user={this.props.email}
          orgId={this.props.orgId} />
        {this.props.children}
      </div>
    );
  }
}


function mapStateToProps(state, ownProps) {
  return {
    instanceList: Object.keys(state.instances).map(k => state.instances[k]),
    instance: state.instances[ownProps.orgId],
    email: state.email,
    instancesMenuOpen: state.instancesMenuOpen,
  };
}


export default connect(mapStateToProps, { getInstance, requestInstancesMenuChange })(PrivatePage);
