import React from 'react';
import FlatButton from 'material-ui/FlatButton';
import { red900 } from 'material-ui/styles/colors';
import List, { ListItem } from 'material-ui/List';
import { hashHistory } from 'react-router';

import { Box } from '../../components/box';
import { encodeURIs } from '../../common/request';
import { getOrganizations } from '../../common/api';
import { trackException, trackView } from '../../common/tracking';

export default class IntancesList extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      loading: true,
      instances: []
    };

    this.renderInstance = this.renderInstance.bind(this);
    this.onGetInstancesSuccess = this.onGetInstancesSuccess.bind(this);
    this.onGetInstancesError = this.onGetInstancesError.bind(this);
  }

  componentDidMount() {
    getOrganizations()
      .then(this.onGetInstancesSuccess, this.onGetInstancesError);
    trackView('InstanceList');
  }

  onClickNew(ev) {
    ev.preventDefault();
    hashHistory.push('/instances/create');
  }

  onGetInstancesSuccess(resp) {
    this.setState({
      loading: false,
      instances: resp.organizations
    });
  }

  onGetInstancesError(resp) {
    const err = resp.errors[0];
    trackException(err.message);
    this.setState({
      errorText: 'Could not load instance list, please try again later.'
    });
  }

  renderAttached(username) {
    return (
      <span>Attached to <span style={{fontWeight: 'bold'}}>{username}</span></span>
    );
  }

  selectInstance(id) {
    hashHistory.push(encodeURIs`/instances/select/${id}`);
  }

  renderInstance(instance) {
    const selectInstance = () => this.selectInstance(instance.name);
    const link = (<FlatButton onClick={selectInstance} label="Select" />);
    const secondaryText = instance.label !== instance.name ? instance.name : false;
    return (
      <ListItem
        style={{cursor: 'default'}}
        key={instance.name}
        primaryText={instance.label}
        rightIconButton={link}
        secondaryText={secondaryText}
      />
    );
  }

  renderInstances() {
    return (
      <Box>
        <List>
          {this.state.instances.map(this.renderInstance)}
        </List>
      </Box>
    );
  }

  render() {
    const styles = {
      createNew: {
        marginTop: 16,
        fontSize: '0.8rem'
      },

      error: {
        display: this.state.errorText ? 'block' : 'none'
      },

      errorLabel: {
        fontSize: '0.8rem',
        color: red900
      },

      errorIcon: {
        marginLeft: 16,
        marginRight: 16
      },

      heading: {
        fontSize: 18,
        textTransform: 'uppercase',
        marginBottom: 36
      }
    };

    return (
      <div>
        <div style={styles.heading}>
          Instances
        </div>
        <p>Choose the instance you want to access:</p>
        {this.state.loading ?
          <span><span className="fa fa-loading" /> Loading...</span> :
          this.state.instances && this.renderInstances()}

        <div style={styles.createNew}>
          Do you have a new cluster? <a href="/instances/create"
            onClick={this.onClickNew}>
            Create a new instance
          </a>
        </div>
        <div style={styles.error}>
          <p style={styles.errorLabel}>
            <span className="fa fa-exclamation" style={styles.errorIcon}></span>
            {this.state.errorText}
          </p>
        </div>
      </div>
    );
  }

}
