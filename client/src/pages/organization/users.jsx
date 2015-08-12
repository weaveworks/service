import React from "react";
import { FlatButton, List, ListItem, RaisedButton, TextField } from "material-ui";
import { getData, deleteData, postData } from "../../common/request";
import { Box } from "../../components/box";

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
    let url = `/api/org/${this.props.org}/users`;
    getData(url)
      .then(resp => {
        this.setState({
          users: resp
        });
      }.bind(this), resp => {
        console.error(resp);
      });
  }

  render() {
    let users = this.state.users.map(user => {
      let buttonStyle = {
        marginTop: 6
      };
      let button = (
        <FlatButton label="Remove" style={buttonStyle} onClick={this._handleDeleteTouchTap.bind(this, user)} />
      );

      return (
        <ListItem primaryText={user.email} key={user.email}
          rightIconButton={button}
        />
      );
    });

    let buttonStyle = {
      marginLeft: '2em'
    };

    let formStyle = {
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
        // <div style={formStyle}>
        //   <TextField hintText="Email" ref="emailField" />
        //   <RaisedButton label="Invite" style={buttonStyle} onClick={this._handleInviteTouchTap.bind(this)} />
        // </div>
      </div>
    );
  }


  // _handleDeleteTouchTap(user) {
  //   let url = `/api/org/${this.props.org}/users/${user.id}`;

  //   deleteData(url)
  //     .then(function(resp) {
  //       this.setState({
  //         users: resp
  //       });
  //       this.refs.emailField.setValue("");
  //     }.bind(this), function(resp) {
  //       // TODO show error
  //     }.bind(this));
  // }

  // _handleInviteTouchTap() {
  //   let url = `/api/org/${this.props.org}/users`;

  //   const email = this.refs.emailField.getValue();

  //   if (email) {
  //     postData(url, {email: email})
  //       .then(function(resp) {
  //         this.setState({
  //           users: resp
  //         });
  //         this.refs.emailField.setValue("");
  //       }.bind(this), function(resp) {
  //         // TODO show error
  //       }.bind(this));
  //     }
  // }

}
