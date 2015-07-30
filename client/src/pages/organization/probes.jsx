import React from "react";
import { Paper, List, ListItem } from "material-ui";
import { getData, postData } from "../../common/request";
import { Box } from "../../components/box";

export default class Probes extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      probes: []
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
