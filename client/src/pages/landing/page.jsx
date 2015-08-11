import React from "react";
import { HashLocation } from "react-router";
import { getData } from "../../common/request";
import { CircularProgress, Styles } from "material-ui";

const Colors = Styles.Colors;
const ThemeManager = new Styles.ThemeManager();

export default class LandingPage extends React.Component {

  static childContextTypes = {
    muiTheme: React.PropTypes.object
  }

  constructor(props) {
    super(props);

    this.state = {
      activityText: 'Checking login state...',
      errorText: ''
    };

    this._checkCookie = this._checkCookie.bind(this);
    this._handleLoginSuccess = this._handleLoginSuccess.bind(this);
    this._handleLoginError = this._handleLoginError.bind(this);
  }

  getChildContext() {
    return {
      muiTheme: ThemeManager.getCurrentTheme()
    };
  }

  componentWillMount() {
    ThemeManager.setPalette({
      accent1Color: Colors.deepOrange500
    });
  }

  componentDidMount() {
    this._checkCookie();
  }

  render() {
    const styles = {
      container: {
        textAlign: 'center',
        paddingTop: '200px'
      },

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
      <div style={styles.container}>
        <h1>Scope as a Service</h1>
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

  _checkCookie() {
    const url = `/api/users/lookup`;
    getData(url).then(this._handleLoginSuccess, this._handleLoginError);
  }

  _handleLoginSuccess(resp) {
    const url = `/org/${resp.organizationName}`;
    this.setState({
      activityText: 'Logged in. Please wait for your app to load...'
    });
    HashLocation.push(url);
  }

  _handleLoginError(resp) {
    if (resp.status === 401) {
      // if unauthorized, send to login page
      this.setState({
        activityText: 'Not logged in. Please wait for the login form to load...'
      });
      HashLocation.push('/login');
    } else {
      let err = resp.errors[0];
      this.setState({
        activityText: '',
        errorText: err.message
      });
    }
  }
}
