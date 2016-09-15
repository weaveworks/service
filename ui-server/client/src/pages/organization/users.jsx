/* eslint react/jsx-boolean-value: 0 */

import React from 'react';
import ReactDOM from 'react-dom';
import { connect } from 'react-redux';
import FlatButton from 'material-ui/FlatButton';
import List, { ListItem } from 'material-ui/List';
import RaisedButton from 'material-ui/RaisedButton';
import Snackbar from 'material-ui/Snackbar';
import TextField from 'material-ui/TextField';

import { getOrganizationUsers } from '../../actions';
import { deleteData, postData, encodeURIs } from '../../common/request';
import { Box } from '../../components/box';


class Users extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
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

  componentDidMount() {
    this.props.getOrganizationUsers(this.props.instance.id);
  }

  getEmailField() {
    const wrapperNode = ReactDOM.findDOMNode(this.refs.emailField);
    return wrapperNode.getElementsByTagName('input')[0];
  }

  clearErrors() {
    this.setState({ notices: null });
  }

  doSubmit() {
    const url = encodeURIs`/api/users/org/${this.props.instance.id}/users`;
    const email = this.getEmailField().value;

    if (email) {
      this.setState({
        submitting: true,
        notices: null,
      });
      postData(url, { email })
        .then(() => {
          this.getEmailField().value = '';
          this.props.getOrganizationUsers(this.props.instance.id);
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
    const url = encodeURIs`/api/users/org/${this.props.instance.id}/users/${user.email}`;

    deleteData(url)
      .then(() => {
        this.props.getOrganizationUsers(this.props.instance.id);
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
        {!this.props.instance.users && <span>
          <span className="fa fa-circle-o-notch fa-spin" /> Loading...
        </span>}
        {this.props.instance.users && <Box>
          <List>
            {this.props.instance.users.map(this.renderUser)}
          </List>
        </Box>}
      </div>
    );
  }
}


export default connect(null, { getOrganizationUsers })(Users);
