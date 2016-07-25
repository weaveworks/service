/* eslint react/jsx-boolean-value: 0 */

import React from 'react';
import ReactDOM from 'react-dom';
import FlatButton from 'material-ui/FlatButton';
import List, { ListItem } from 'material-ui/List';
import RaisedButton from 'material-ui/RaisedButton';
import Snackbar from 'material-ui/Snackbar';
import TextField from 'material-ui/TextField';
import debug from 'debug';

import { getData, deleteData, postData, encodeURIs } from '../../common/request';
import { Box } from '../../components/box';

const error = debug('service:usersErr');


export default class Users extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      users: [],
      submitting: false,
      notices: null,
    };
    this.doSubmit = this.doSubmit.bind(this);
    this.clearErrors = this.clearErrors.bind(this);
    this.renderUser = this.renderUser.bind(this);

    this.handleKeyDown = this.handleKeyDown.bind(this);
    this.handleInviteTouchTap = this.handleInviteTouchTap.bind(this);
    this.handleDeleteTouchTap = this.handleDeleteTouchTap.bind(this);
  }

  componentWillMount() {
    this.getUsers();
  }

  getEmailField() {
    const wrapperNode = ReactDOM.findDOMNode(this.refs.emailField);
    return wrapperNode.getElementsByTagName('input')[0];
  }

  getUsers() {
    const url = encodeURIs`/api/users/org/${this.props.org}/users`;
    getData(url)
      .then(resp => {
        this.setState({
          users: resp.users
        });
      }, resp => {
        error(resp);
      });
  }

  clearErrors() {
    this.setState(Object.assign({}, this.state, {notices: null}));
  }

  doSubmit() {
    const url = encodeURIs`/api/users/org/${this.props.org}/users`;
    const email = this.getEmailField().value;

    if (email) {
      this.setState({
        submitting: true,
        notices: null,
      });
      postData(url, { email })
        .then(() => {
          this.getEmailField().value = '';
          this.getUsers();
          this.setState({
            notices: [{message: `Invitation sent to ${email}`}],
            submitting: false,
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

  handleDeleteTouchTap(user) {
    const url = encodeURIs`/api/users/org/${this.props.org}/users/${user.email}`;

    deleteData(url)
      .then(() => {
        this.getUsers();
        this.getEmailField().value = '';
      }, resp => {
        this.setState({
          notices: resp.errors
        });
      });
  }

  handleInviteTouchTap() {
    this.doSubmit();
  }

  renderUser(user) {
    const buttonStyle = {
      marginTop: 6
    };
    const deleteUser = () => this.handleDeleteTouchTap(user);
    const button = user.self ? (<FlatButton
      label="Self"
      style={buttonStyle}
      disabled />) :
      (<FlatButton
        label="Remove"
        style={buttonStyle}
        onClick={deleteUser}
        />);

    return (
      <ListItem primaryText={user.email} key={user.email} style={{cursor: 'default'}}
        rightIconButton={button}
      />
    );
  }

  render() {
    const users = this.state.users.map(this.renderUser);

    const buttonStyle = {
      marginLeft: '2em'
    };

    const formStyle = {
      marginTop: '1em'
    };

    return (
      <div className="users">
        <Snackbar
          action="ok"
          open={Boolean(this.state.notices)}
          message={this.state.notices ? this.state.notices.map(e => e.message).join('. ') : ''}
          onActionTouchTap={this.clearErrors}
          onRequestClose={this.clearErrors}
        />
        <div style={formStyle}>
          <TextField
            hintText="Email"
            ref="emailField"
            disabled={Boolean(this.state.submitting)}
            onKeyDown={this.handleKeyDown}
            />
          <RaisedButton
            label="Invite"
            disabled={Boolean(this.state.submitting)}
            style={buttonStyle}
            onClick={this.handleInviteTouchTap}
            />
        </div>
        <h4>Current members</h4>
        <Box>
          <List>
            {users}
          </List>
        </Box>
      </div>
    );
  }

}
