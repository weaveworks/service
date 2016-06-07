import React from 'react';
import { FlatButton } from 'material-ui';

import { FlexContainer } from '../../components/flex-container';
import { Column } from '../../components/column';
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
      <FlexContainer key={a.id} title={a.title} style={{padding: 0, alignItems: 'center'}}>
        <Column style={{margin: '0 36px 0 0'}}><span className={a.icon} /> {a.name}</Column>
        <Column>{a.username}</Column>
        <Column style={{margin: '0 0 0 36px'}}>{link}</Column>
      </FlexContainer>
    );
  }

  render() {
    return (
      <div>
        <h2>External Logins</h2>
        <p>Control which external accounts are attached to this Weave Scope user.</p>
        {(this.state.logins || []).map(this.renderLogin)}
      </div>
    );
  }

}
