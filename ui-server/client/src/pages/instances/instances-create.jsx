/* eslint react/jsx-no-bind: 0, no-return-assign: 0 */

import React from 'react';
import FlatButton from 'material-ui/FlatButton';
import RaisedButton from 'material-ui/RaisedButton';
import TextField from 'material-ui/TextField';
import { red900 } from 'material-ui/styles/colors';
import { hashHistory } from 'react-router';

import { getNewInstanceName } from '../../common/api';
import { encodeURIs, postData } from '../../common/request';
import { trackException, trackView } from '../../common/tracking';


const DEFAULT_LABEL = 'Untitled Cluster';

export default class InstancesCreate extends React.Component {

  constructor(props) {
    super(props);

    const { first } = this.props.params;

    this.state = {
      first,
      name: '',
      label: DEFAULT_LABEL,
      errorText: '',
      instanceCreated: false,
      submitText: 'Loading instance...',
      submitting: false
    };

    this._handleSubmit = this._handleSubmit.bind(this);
    this.handleNewInstanceNameSuccess = this.handleNewInstanceNameSuccess.bind(this);
    this.handleNewInstanceNameError = this.handleNewInstanceNameError.bind(this);
    this.handleChangeLabel = this.handleChangeLabel.bind(this);
    this.handleChangeName = this.handleChangeName.bind(this);
    this.handleCancel = this.handleCancel.bind(this);
  }

  componentDidMount() {
    this.initializeInstance();
    trackView('InstanceCreate');
  }

  initializeInstance() {
    getNewInstanceName()
      .then(this.handleNewInstanceNameSuccess, this.handleNewInstanceNameError);
  }

  handleNewInstanceNameError(resp) {
    const err = resp.errors[0];
    trackException(err.message);
    this.setState({
      errorText: 'Could not acquire instance, please try again later.'
    });
  }

  handleNewInstanceNameSuccess(resp) {
    this.setState({
      submitText: this.state.first ? 'Continue' : 'Create',
      name: resp.name
    });
  }

  handleChangeLabel(ev) {
    this.setState({label: ev.target.value});
  }

  handleChangeName(ev) {
    this.setState({name: ev.target.value});
  }

  handleCancel() {
    if (this.props.params.first) {
      hashHistory.push('/logout');
    } else {
      window.history.back();
    }
  }

  _handleSubmit() {
    const { label, name } = this.state;
    const errorTextLabel = label ? '' : 'Label cannot be empty.';
    const errorTextName = name ? '' : 'Name cannot be empty.';

    if (!label || !name) {
      this.setState({ errorTextName, errorTextLabel });
      return;
    }

    // lock button and clear error
    this.setState({
      errorTextName: '',
      errorTextLabel: '',
      errorText: '',
      submitting: true,
      submitText: 'Creating...'
    });

    postData('/api/users/org', {label, name})
      .then(() => {
        hashHistory.push(encodeURIs`/instances/select/${this.state.name}`);
      }, resp => {
        const err = resp.errors[0];
        this.setState({
          errorText: err.message,
          submitting: false,
          submitText: 'Create'
        });
        trackException(resp);
      });
  }

  render() {
    const submitSuccess = this.state.instanceCreated;
    const { first } = this.state;

    const heading = first
      ? 'Welcome to your instance'
      : 'Create a new instance';

    const submitText = first ? 'Continue' : this.state.submitText;

    const styles = {
      submit: {
        marginLeft: '1em',
        marginTop: 24,
        verticalAlign: 'top'
      },

      error: {
        display: this.state.errorText ? 'block' : 'none'
      },

      errorLabel: {
        fontSize: '0.8rem',
        color: red900
      },

      errorIcon: {
        marginLeft: 16,
        marginRight: 16
      },

      form: {
        width: 550,
        display: !submitSuccess ? 'block' : 'none'
      },

      formHint: {
        marginTop: '0.25em',
        fontSize: '0.7rem',
        opacity: 0.6
      }
    };

    return (
      <div>
        <h2>
          {heading}
        </h2>
        <div style={styles.form}>
          <p>Let's start by creating a monitoring instance for your cluster.
            <br />Give your cluster a name:</p>
          <TextField hintText="Provide a label" floatingLabelText="Label"
            disabled={this.state.submitting}
            onChange={this.handleChangeLabel}
            style={{verticalAlign: 'top', width: 400}}
            value={this.state.label}
            errorText={this.state.errorTextLabel} />

          <div style={styles.error}>
            <p style={styles.errorLabel}>
              <span className="fa fa-exclamation" style={styles.errorIcon}></span>
              {this.state.errorText}
            </p>
          </div>

          <div style={styles.formHint}>
            Your monitoring instance will have the ID {this.state.name || '...'}
          </div>

          <div style={styles.formButtons}>
            <RaisedButton primary label={submitText} style={styles.submit}
              disabled={this.state.submitting || !this.state.name} onClick={this._handleSubmit} />
            <FlatButton label="Cancel" style={styles.submit}
              disabled={this.state.submitting} onClick={this.handleCancel} />
          </div>

        </div>
      </div>
    );
  }

}
