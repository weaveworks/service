import React from 'react';
import Colors from '../../common/colors';
import RaisedButton from 'material-ui/RaisedButton';

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
    const logins = injectPrefix(this.props.logins, this.props.prefix);
    return (
      <span>
        {(logins || []).map(a =>
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
