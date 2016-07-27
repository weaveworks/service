import React from 'react';
import { hashHistory } from 'react-router';

import Dialog from 'material-ui/Dialog';
import FlatButton from 'material-ui/FlatButton';
import RaisedButton from 'material-ui/RaisedButton';
import TextField from 'material-ui/TextField';
import { red900 } from 'material-ui/styles/colors';

import { trackException } from '../../common/tracking';
import { deleteData, encodeURIs } from '../../common/request';

export default class InstancesDelete extends React.Component {

  constructor() {
    super();
    this.state = {
      deleting: false,
      deleteDialogOpen: false,
      deleteDialogValid: false,
      deleteErrorText: ''
    };

    this.handleOpenDeleteDialog = this.handleOpenDeleteDialog.bind(this);
    this.handleCloseDeleteDialog = this.handleCloseDeleteDialog.bind(this);
    this.handleClickDelete = this.handleClickDelete.bind(this);
    this.handleChangeDeleteInput = this.handleChangeDeleteInput.bind(this);
    this.handleDeleteSuccess = this.handleDeleteSuccess.bind(this);
    this.handleDeleteError = this.handleDeleteError.bind(this);
  }

  handleCloseDeleteDialog() {
    this.setState({
      deleteDialogValid: false,
      deleteDialogOpen: false
    });
  }

  handleOpenDeleteDialog() {
    this.setState({
      deleteDialogValid: false,
      deleteDialogOpen: true
    });
  }

  handleClickDelete() {
    this.setState({ deleting: true, deleteDialogOpen: false });
    deleteData(encodeURIs`/api/users/org/${this.props.params.orgId}`)
      .then(this.handleDeleteSuccess)
      .catch(this.handleDeleteError);
  }

  handleDeleteSuccess() {
    hashHistory.push('/instances/deleted');
  }

  handleDeleteError(resp) {
    const err = resp.errors[0];
    trackException(err.message);
    this.setState({
      deleting: false,
      deleteErrorText: err.message
    });
  }

  handleChangeDeleteInput(ev) {
    const deleteDialogValid = this.state.instanceName
      && this.state.instanceName === ev.target.value;
    this.setState({ deleteDialogValid });
  }

  render() {
    const styles = {
      deleteError: {
        display: this.state.deleteErrorText ? 'block' : 'none'
      },
      errorLabel: {
        fontSize: '0.8rem',
        color: red900
      },
      heading: {
        textTransform: 'uppercase'
      }
    };

    const { instanceName } = this.props;
    const deleteDialogTitle = `Delete instance ${instanceName}`;
    const deleteDialogActions = [
      <FlatButton primary
        label="Delete"
        disabled={!this.state.deleteDialogValid}
        onTouchTap={this.handleClickDelete}
      />,
      <FlatButton keyboardFocused
        label="Cancel"
        onTouchTap={this.handleCloseDeleteDialog}
      />
    ];

    return (
      <div style={this.props.style}>
        <div style={styles.heading}>Delete this instance</div>
        <p>You can delete this Weave Cloud instance for your cluster {instanceName}</p>
        <div style={styles.deleteError}>
          <p style={styles.errorLabel}>
            {this.state.deleteErrorText}
          </p>
        </div>
        <RaisedButton
          style={{ top: 18, right: 18 }}
          disabled={this.state.deleting}
          onClick={this.handleOpenDeleteDialog}
          label="Delete this instance" />

        <Dialog
          title={deleteDialogTitle}
          actions={deleteDialogActions}
          modal={false}
          open={this.state.deleteDialogOpen}
          onRequestClose={this.handleCloseDeleteDialog}
        >
          To delete this instance, please type its name into the field:
          <br />
          <TextField
            hintText="Type the instance name"
            onChange={this.handleChangeDeleteInput}
            />
        </Dialog>
      </div>
    );
  }

}
