import React from 'react';
import FlatButton from 'material-ui/FlatButton';
import { red900 } from 'material-ui/styles/colors';
import List, { ListItem } from 'material-ui/List';
import { hashHistory } from 'react-router';

import { Box } from '../../components/box';
import { encodeURIs } from '../../common/request';
import { getOrganizations } from '../../common/api';
import { trackException } from '../../common/tracking';

export default class IntancesList extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      loading: true,
      instances: []
    };

    this.renderInstance = this.renderInstance.bind(this);
    this.handleGetInstancesSuccess = this.handleGetInstancesSuccess.bind(this);
    this.handleGetInstancesError = this.handleGetInstancesError.bind(this);
  }

  componentDidMount() {
    getOrganizations()
      .then(this.handleGetInstancesSuccess, this.handleGetInstancesError);
  }

  onClickNew(ev) {
    ev.preventDefault();
    hashHistory.push('/instances/create');
  }

  handleGetInstancesSuccess(resp) {
    this.setState({
      loading: false,
      instances: resp.organizations
    });
  }

  handleGetInstancesError(resp) {
    const err = resp.errors[0];
    trackException(err.message);
    this.setState({
      errorText: 'Could not load instance list, please try again later.'
    });
  }

  selectInstance(id) {
    hashHistory.push(encodeURIs`/instances/select/${id}`);
  }

  renderInstance(instance) {
    const selectInstance = () => this.selectInstance(instance.name);
    const isCurrentInstance = this.props.currentInstance === instance.name;
    const link = isCurrentInstance ? (<FlatButton label="Active" disabled />) : (<FlatButton
      onClick={selectInstance} label="Select" />);
    return (
      <ListItem disabled
        style={{cursor: 'default'}}
        key={instance.name}
        primaryText={instance.label}
        rightIconButton={link}
        secondaryText={instance.name}
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
      }
    };

    return (
      <div>
        {this.state.loading ?
          <span><span className="fa fa-loading" /> Loading...</span> :
          this.state.instances && this.renderInstances()}

        <div style={styles.createNew}>
          Do you have a new cluster? <a href="/instances/create"
            onClick={this.onClickNew}>
            Create a new instance
          </a> to monitor it.
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
