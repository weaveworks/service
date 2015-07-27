import React from "react";
import { getData, postData } from "../../common/request";
import { Styles, RaisedButton, TextField } from "material-ui";

const ThemeManager = new Styles.ThemeManager();

export default class Users extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      users: []
    };
  }

  static childContextTypes = {
    muiTheme: React.PropTypes.object
  }

  getChildContext() {
    return {
      muiTheme: ThemeManager.getCurrentTheme()
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
      }.bind(this));
  }

  render() {
    let users = this.state.users.map(user => {
      return (
        <li>{user.email}</li>
      );
    });

    let buttonStyle = {
      marginLeft: '2em'
    };


    return (
      <div className="users">
        <h3>Users</h3>
        <div>
          <ul>
            {users}
          </ul>
          <TextField hintText="Email" ref="emailField" />
          <RaisedButton label="Invite" style={buttonStyle} onClick={this._handleTouchTap.bind(this)} />
        </div>
      </div>
    );
  }

  _handleTouchTap() {
    let url = `/api/org/${this.props.org}/users`;

    const email = this.refs.emailField.getValue();

    if (email) {
      postData(url, {email: email})
        .then(function(resp) {
          this.setState({
            users: resp
          });
          this.refs.emailField.setValue("");
        }.bind(this), function(resp) {
          // TODO show error
        }.bind(this));
      }
  }

}
