import React from "react";
import { HashLocation } from "react-router";
import { getData, postData } from "../../common/request";
import { CircularProgress, Styles } from "material-ui";

const Colors = Styles.Colors;

export default class LoginForm extends React.Component {

  constructor(props) {
    super(props);

    this.state = {
      activityText: 'Logging out...',
      errorText: ''
    };

    this._handleSuccess = this._handleSuccess.bind(this);
    this._handleError = this._handleError.bind(this);
  }

  componentDidMount() {
    this._tryLogout();
  }

  render() {
    const styles = {
      error: {
        display: this.state.errorText ? "block" : "none",
        fontSize: '85%',
        opacity: 0.6,
        color: Styles.Colors.red900
      },

      activity: {
        display: this.state.activityText ? "block" : "none",
        fontSize: '85%',
        opacity: 0.6
      }
    };

    return (
      <div>
        <div style={styles.activity}>
          <CircularProgress mode="indeterminate" />
          <p>{this.state.activityText}.</p>
        </div>
        <div style={styles.error}>
          <h3>Scope service is not available. Please try again later.</h3>
          <p>{this.state.errorText}</p>
        </div>
      </div>
    );
  }

  _tryLogout() {
    const url = `/api/users/logout`;
    getData(url).then(this._handleSuccess, this._handleError);
  }

  _handleSuccess(resp) {
    HashLocation.push('/');
  }

  _handleError(resp) {
    this.setState({
      activityText: '',
      errorText: resp.errors[0].message
    });
  }
}
