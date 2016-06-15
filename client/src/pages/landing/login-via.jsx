import React from 'react';
import Colors from '../../common/colors';
import { RaisedButton } from 'material-ui';
import { getLogins } from '../../common/api';
import { trackException, trackView } from '../../common/tracking';

function injectPrefix(logins, prefix) {
  if (prefix) {
    return (logins || []).map(l => {
      l.link.label = `${prefix} ${l.link.label}`;
      return l;
    });
  }
  return logins;
}

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
    getLogins().then(this._handleLoadSuccess, this._handleLoadError);
  }

  _handleLoadSuccess(resp) {
    const logins = injectPrefix(resp.logins, this.props.prefix);
    this.setState({ logins });
  }

  _handleLoadError(resp) {
    trackException(resp);
  }

  render() {
    const styles = {
      base: {
        marginTop: '3px',
        verticalAlign: 'top'
      },
      wrapper: {
        marginRight: '1em'
      }
    };
    return (
      <span>
        {(this.state.logins || []).map(a =>
            <span key={a.link.href} className="login-via" style={styles.wrapper}>
              <RaisedButton linkButton
                style={styles.base}
                labelColor={Colors.white}
                {...a.link}
                icon={<span className={a.link.icon}></span>}
              />
            </span>
        )}
        </span>
    );
  }
}
