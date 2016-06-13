import React from 'react';
import _ from 'lodash';
import { FlatButton } from 'material-ui';

import { FlexContainer } from '../../components/flex-container';
import { Column } from '../../components/column';
import { getData, postData, encodeURIs } from '../../common/request';
import { trackException } from '../../common/tracking';

export default class Logins extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      loading: true,
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
    this.setState({ loading: true });

    Promise.all([
      getData('/api/users/logins'),
      getData('/api/users/attached_logins')
    ]).then(([loginsRes, attachedLoginsRes]) => {
      const logins = loginsRes.logins || [];
      const attachedLogins = attachedLoginsRes.logins || [];
      const attachedIndex = _.fromPairs(attachedLogins.map(l => [l.id, l]));

      this.setState({
        loading: false,
        logins: logins.map(l => _.merge(l, attachedIndex[l.id])),
      });
    }, resp => {
      this.setState({ loading: false });
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

  renderAttached(username) {
    return (
      <span>Attached to <span style={{fontWeight: 'bold'}}>{username}</span></span>
    );
  }

  renderLogin(a) {
    const detach = () => this.detach(a.id);
    let link = <FlatButton linkButton href={a.link.href} label="Attach" />;
    if (a.loginID || a.username) {
      link = <FlatButton onClick={detach} label="Detach" />;
    }
    const style = {borderBottom: '1px solid #aaa', padding: 0, alignItems: 'center'};
    return (
      <FlexContainer key={a.id} style={style}>
        <Column style={{margin: '0 36px 0 0'}}><span className={a.link.icon} /> {a.name}</Column>
        <Column>{a.username ? this.renderAttached(a.username) : 'Not attached'}
        </Column>
        <Column style={{textAlign: 'right', margin: '0 0 0 36px'}}>{link}</Column>
      </FlexContainer>
    );
  }

  render() {
    return (
      <div>
        <h2>External Logins</h2>
        <p>Control which external accounts are attached to this user.</p>
        {this.state.loading ?
          <span><span className="fa fa-loading" /> Loading...</span> :
          (this.state.logins || []).map(this.renderLogin)}
      </div>
    );
  }

}
