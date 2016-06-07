import React from 'react';
import { FlatButton } from 'material-ui';

import { getData, postData, encodeURIs } from '../../common/request';
import { trackException } from '../../common/tracking';

export default class Logins extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      logins: [],
    };
    this.getLogins = this.getLogins.bind(this);

    this.renderLogin = this.renderLogin.bind(this);
    this.detach = this.detach.bind(this);
  }

  componentDidMount() {
    this.getLogins();
  }

  getLogins() {
    const url = encodeURIs`/api/users/logins`;
    getData(url)
      .then(resp => {
        this.setState(resp);
      }, resp => {
        trackException(resp.errors[0].message);
      });
  }

  detach(id) {
    postData(encodeURIs`/api/users/logins/${id}/detach`, null)
      .then(() => {
        this.getLogins();
      }, resp => {
        trackException(resp.errors[0].message);
      });
  }

  renderLogin(a) {
    const detach = () => this.detach(a.id);
    let link = <FlatButton linkButton href={a.href} label="Attach" />;
    if (a.loginID || a.username) {
      link = <FlatButton onClick={detach} label="Detach" />;
    }
    return (
      <div key={a.id} style={a.style} title={a.title}>
        {a.name} {a.username} {link}
      </div>
    );
  }

  render() {
    return <div>{(this.state.logins || []).map(this.renderLogin)}</div>;
  }

}
