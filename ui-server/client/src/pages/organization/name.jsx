/* eslint react/jsx-boolean-value: 0 */

import React from 'react';
import FlatButton from 'material-ui/FlatButton';
import Snackbar from 'material-ui/Snackbar';
import TextField from 'material-ui/TextField';

import { putData, encodeURIs } from '../../common/request';

export default class Users extends React.Component {

  constructor(props) {
    super(props);

    this.state = {
      editing: false,
      editingName: props.name,
      submitting: false,
      notices: null,
    };

    this.doSubmit = this.doSubmit.bind(this);
    this.clearErrors = this.clearErrors.bind(this);
    this.handleKeyDown = this.handleKeyDown.bind(this);
    this.handleClickEdit = this.handleClickEdit.bind(this);
    this.handleClickCancel = this.handleClickCancel.bind(this);
    this.handleClickSave = this.handleClickSave.bind(this);
    this.handleChangeNameInput = this.handleChangeNameInput.bind(this);
  }

  clearErrors() {
    this.setState({ notices: null });
  }

  doSubmit() {
    const url = encodeURIs`/api/users/org/${this.props.id}`;
    const name = this.state.editingName;

    if (name) {
      this.setState({
        submitting: true,
        notices: null,
      });
      putData(url, { name })
        .then(() => {
          this.setState({
            notices: [{message: `Name changed to ${name}`}],
            submitting: false,
            editing: false
          });
        }, resp => {
          this.setState({
            notices: resp.errors,
            submitting: false,
          });
        });
    }
  }

  handleKeyDown(ev) {
    if (ev.keyCode === 13) { // ENTER
      this.doSubmit();
    }
  }

  handleClickCancel() {
    this.setState({
      editing: false,
      editingName: this.props.name
    });
  }

  handleClickEdit() {
    this.setState({ editing: true });
  }

  handleClickSave() {
    this.doSubmit();
  }

  handleChangeNameInput(ev) {
    this.setState({ editingName: ev.target.value });
  }

  render() {
    const { editing } = this.state;

    const styles = {
      button: {
        marginLeft: '0.5em'
      }
    };

    return (
      <span>
        <Snackbar
          action="ok"
          open={Boolean(this.state.notices)}
          message={this.state.notices ? this.state.notices.map(e => e.message).join('. ') : ''}
          onActionTouchTap={this.clearErrors}
          onRequestClose={this.clearErrors}
        />
        {editing && <span style={styles.form}>
          <TextField
            onChange={this.handleChangeNameInput}
            value={this.state.editingName}
            hintText="Provide a name"
            onKeyDown={this.handleKeyDown}
            />
          <FlatButton primary
            label="Save"
            disabled={Boolean(this.state.submitting)}
            style={styles.button}
            onClick={this.handleClickSave}
            />
          <FlatButton
            label="Cancel"
            disabled={Boolean(this.state.submitting)}
            style={styles.button}
            onClick={this.handleClickCancel}
            />
          </span>}
        {!editing && <span>
          {this.props.prefix} {this.state.editingName || this.props.name}
          <FlatButton
            label="Edit"
            style={styles.button}
            onClick={this.handleClickEdit}
            />
          </span>}
      </span>
    );
  }

}
