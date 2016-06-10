import React from 'react';
import Colors from '../../common/colors';
import { RaisedButton } from 'material-ui';
import { getData } from '../../common/request';
import { trackException, trackView } from '../../common/tracking';

export default class LoginVia extends React.Component {

  constructor(props) {
    super(props);

    this.state = {};

    this._handleLoadSuccess = this._handleLoadSuccess.bind(this);
    this._handleLoadError = this._handleLoadError.bind(this);
  }

  componentDidMount() {
    this._load();
    trackView('LoginVia');
  }

  _load() {
    const url = '/api/users/logins';
    getData(url).then(this._handleLoadSuccess, this._handleLoadError);
  }

  _handleLoadSuccess(resp) {
    this.setState(resp);
  }

  _handleLoadError(resp) {
    trackException(resp);
  }

  render() {
    const styles = {
      base: {
        color: Colors.white,
        marginLeft: '2em',
        marginTop: '3px',
        verticalAlign: 'top'
      }
    };
    return (
      <div>
        {(this.state.logins || []).map(a =>
            <div key={a.href}>
              <RaisedButton linkButton secondary
                style={styles.base}
                {...a}
                icon={<span className={a.icon}></span>}
              />
            </div>
        ) }
        </div>
    );
  }
}
