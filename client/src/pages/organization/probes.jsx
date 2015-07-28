import React from "react";
import { Styles, Paper, List, ListItem } from "material-ui";
import { getData, postData } from "../../common/request";
import { Box } from "../../components/box";

const ThemeManager = new Styles.ThemeManager();

export default class Probes extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      probes: []
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
    this.getProbes();
  }

  getProbes() {
    let url = `/api/org/${this.props.org}/probes`;
    getData(url)
      .then(resp => {
        this.setState({
          probes: resp
        });
      }.bind(this));
  }

  render() {
    let probes = this.state.probes.map(probe => {
      return (
        <ListItem primaryText={probe.id} key={probe.id} />
      );
    });

    return (
      <Box>
        <List subheader="Probes">
          {probes}
        </List>
      </Box>
    );
  }

}
