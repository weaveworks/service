import React from 'react';
import Paper from 'material-ui/Paper';
import Menu from 'material-ui/Menu';
import Subheader from 'material-ui/Subheader';

export default class MenuSection extends React.Component {
  render() {
    const styles = {
      section: {
        padding: 24
      }
    };

    return (
      <Paper style={{marginTop: '4em', marginBottom: '1em'}}>
        <div style={styles.section}>
        <Menu>
          {this.props.title && <Subheader>{this.props.title}</Subheader>}
          {this.props.children}
        </Menu>
        </div>
      </Paper>
    );
  }
}
