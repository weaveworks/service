import React from 'react';
import CircularProgress from 'material-ui/CircularProgress';

import MenuLink from './link';
import { trackException } from '../../common/tracking';
import { encodeURIs, getData } from '../../common/request';

export default class RemoteMenuLinks extends React.Component {
  constructor() {
    super();

    this.state = {
      loading: true,
      links: [],
    };

    this._load = this._load.bind(this);
    this.handleLoadSuccess = this.handleLoadSuccess.bind(this);
    this.handleLoadError = this.handleLoadError.bind(this);
  }

  componentDidMount() {
    // load the data
    this._load()
      .then(this.handleLoadSuccess)
      .catch(this.handleLoadError);
  }

  _load() {
    return getData(this.props.url);
  }

  handleLoadSuccess(resp) {
    this.setState({
      loading: false,
      links: resp.links,
    });
  }

  handleLoadError(resp) {
    trackException(resp);
  }

  renderLink({href, text}) {
    return <MenuLink to={encodeURIs`${this.props.prefix}${href}`}>{text}</MenuLink>;
  }

  render() {
    const styles = {
      activity: {
        marginTop: 25,
        textAlign: 'center'
      }
    };

    if (this.state.loading) {
      return <div style={styles.activity}><CircularProgress mode="indeterminate" /></div>;
    }
    return <div>this.state.items.map(this.renderLink)</div>;
  }
}

