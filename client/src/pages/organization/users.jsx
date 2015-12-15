import React from 'react';
import { FlatButton, List, ListItem, RaisedButton, TextField } from 'material-ui';
import debug from 'debug';

import { getData, deleteData, postData, encodeURIs } from '../../common/request';
import { Box } from '../../components/box';

const error = debug('service:usersErr');

export default class Users extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      users: []
    };
  }

  componentWillMount() {
    this.getUsers();
  }

  getUsers() {
    const url = encodeURIs`/api/org/${this.props.org}/users`;
    getData(url)
      .then(resp => {
        this.setState({
          users: resp
        });
      }, resp => {
        error(resp);
      });
  }

  _handleDeleteTouchTap(user) {
    const url = encodeURIs`/api/org/${this.props.org}/users/${user.id}`;

    deleteData(url)
      .then(resp => {
        this.setState({
          users: resp
        });
        this.refs.emailField.setValue('');
      }, resp => {
        error(resp);
      });
  }

  _handleInviteTouchTap() {
    const url = encodeURIs`/api/org/${this.props.org}/users`;

    const email = this.refs.emailField.getValue();

    if (email) {
      postData(url, {email: email})
        .then(resp => {
          this.setState({
            users: resp
          });
          this.refs.emailField.setValue('');
        }, resp => {
          error(resp);
        });
    }
  }

  render() {
    const users = this.state.users.map(user => {
      const buttonStyle = {
        marginTop: 6
      };
      const button = (
        <FlatButton label="Remove" style={buttonStyle} onClick={this._handleDeleteTouchTap.bind(this, user)} />
      );

      return (
        <ListItem primaryText={user.email} key={user.email}
          rightIconButton={button}
        />
      );
    });

    const buttonStyle = {
      marginLeft: '2em'
    };

    const formStyle = {
      textAlign: 'center',
      marginTop: '1em'
    };

    return (
      <div className="users">
        <Box>
          <List subheader="Users">
            {users}
          </List>
        </Box>
        <div style={formStyle}>
          <TextField hintText="Email" ref="emailField" />
          <RaisedButton label="Invite" style={buttonStyle} onClick={this._handleInviteTouchTap.bind(this)} />
        </div>
      </div>
    );
  }

}
