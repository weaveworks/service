import React from 'react';
import ReactDOM from 'react-dom';
import { FlatButton, List, ListItem, RaisedButton, TextField } from 'material-ui';
import debug from 'debug';

import { getData, deleteData, postData, encodeURIs } from '../../common/request';
import { Box } from '../../components/box';

const error = debug('service:usersErr');


function renderErrors(errors) {
  return (
    <div>
      {errors.map((e, i) => <div key={i}>{e.message}</div>)}
    </div>
  );
}


export default class Users extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      users: [],
      submitting: false,
      errors: null,
    };
    this.doSubmit = this.doSubmit.bind(this);
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
          users: resp
        });
      }, resp => {
        error(resp);
      });
  }

  doSubmit() {
    const url = encodeURIs`/api/users/org/${this.props.org}/users`;
    const email = this.getEmailField().value;

    if (email) {
      this.setState({
        submitting: true,
      });
      postData(url, { email })
        .then(() => {
          this.getEmailField().value = '';
          this.getUsers();
          this.setState({
            submitting: false,
          });
        }, resp => {
          this.setState({
            errors: resp.errors,
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
          errors: resp.errors
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
      disabled="true" />) :
      (<FlatButton
        label="Remove"
        style={buttonStyle}
        onClick={deleteUser}
        />);

    return (
      <ListItem primaryText={user.email} key={user.email}
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
      textAlign: 'center',
      marginTop: '1em'
    };

    return (
      <div className="users">
        <h3>Current members</h3>
        <Box>
          <List>
            {users}
          </List>
        </Box>
        {this.state.errors && renderErrors(this.state.errors)}
        <div style={formStyle}>
          <TextField
            hintText="Email"
            ref="emailField"
            disabled={this.state.submitting}
            onKeyDown={this.handleKeyDown}
            />
          <RaisedButton
            label="Invite"
            disabled={this.state.submitting}
            style={buttonStyle}
            onClick={this.handleInviteTouchTap}
            />
        </div>
      </div>
    );
  }

}
