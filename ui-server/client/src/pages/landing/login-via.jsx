import React from 'react';
import Colors from '../../common/colors';
import RaisedButton from 'material-ui/RaisedButton';

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
    return (
      <span>
        {(this.props.logins || []).map(a =>
            <span key={a.link.href} className="login-via" style={styles.wrapper}>
              <RaisedButton linkButton
                style={styles.base}
                labelColor={Colors.white}
                backgroundColor={a.link.backgroundColor}
                href={a.link.href}
                label={`${this.props.prefix} ${a.link.label}`}
                icon={<span className={a.link.icon}></span>}
              />
            </span>
        )}
        </span>
    );
  }
}
