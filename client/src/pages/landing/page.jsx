import React from "react";
import { HashLocation, RouteHandler } from "react-router";
import { getData } from "../../common/request";
import { CircularProgress, Styles } from "material-ui";

const Colors = Styles.Colors;
const ThemeManager = new Styles.ThemeManager();

export default class LandingPage extends React.Component {

  static childContextTypes = {
    muiTheme: React.PropTypes.object
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

  render() {
    const styles = {
      container: {
        textAlign: 'center',
        paddingTop: '200px'
      }
    };

    return (
      <div style={styles.container}>
        <h1>Scope as a Service</h1>
        <RouteHandler {...this.props} />
      </div>
    );
  }
}
