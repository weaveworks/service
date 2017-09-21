import React from 'react';
import moment from 'moment';
import classNames from 'classnames';
import { connect } from 'react-redux';
import { debounce } from 'lodash';

import TimeTravelTimeline from './time-travel-timeline';
import { trackMixpanelEvent } from '../utils/tracking-utils';
import { clampToNowInSecondsPrecision } from '../utils/time-utils';
import {
  jumpToTime,
  resumeTime,
  timeTravelStartTransition,
} from '../actions/app-actions';

import { TIMELINE_DEBOUNCE_INTERVAL } from '../constants/timer';


const getTimestampStates = (timestamp) => {
  timestamp = timestamp || moment();
  return {
    inputValue: moment(timestamp).utc().format(),
  };
};

class TimeTravel extends React.Component {
  constructor(props, context) {
    super(props, context);

    this.state = getTimestampStates(props.pausedAt);

    this.handleInputChange = this.handleInputChange.bind(this);
    this.handleTimelinePan = this.handleTimelinePan.bind(this);
    this.handleTimelinePanEnd = this.handleTimelinePanEnd.bind(this);
    this.handleInstantJump = this.handleInstantJump.bind(this);

    this.trackTimestampEdit = this.trackTimestampEdit.bind(this);
    this.trackTimelineClick = this.trackTimelineClick.bind(this);
    this.trackTimelinePan = this.trackTimelinePan.bind(this);

    this.instantUpdateTimestamp = this.instantUpdateTimestamp.bind(this);
    this.debouncedUpdateTimestamp = debounce(
      this.instantUpdateTimestamp.bind(this), TIMELINE_DEBOUNCE_INTERVAL);
  }

  componentWillReceiveProps(props) {
    this.setState(getTimestampStates(props.pausedAt));
  }

  handleInputChange(ev) {
    const timestamp = moment(ev.target.value);
    this.setState({ inputValue: ev.target.value });

    if (timestamp.isValid()) {
      const clampedTimestamp = clampToNowInSecondsPrecision(timestamp);
      this.instantUpdateTimestamp(clampedTimestamp, this.trackTimestampEdit);
    }
  }

  handleTimelinePan(timestamp) {
    this.setState(getTimestampStates(timestamp));
    this.debouncedUpdateTimestamp(timestamp);
  }

  handleTimelinePanEnd(timestamp) {
    this.instantUpdateTimestamp(timestamp, this.trackTimelinePan);
  }

  handleInstantJump(timestamp) {
    this.instantUpdateTimestamp(timestamp, this.trackTimelineClick);
  }

  instantUpdateTimestamp(timestamp, callback) {
    if (!timestamp.isSame(this.props.pausedAt)) {
      this.debouncedUpdateTimestamp.cancel();
      this.setState(getTimestampStates(timestamp));
      this.props.timeTravelStartTransition();
      this.props.jumpToTime(moment(timestamp));

      // Used for tracking.
      if (callback) callback();
    }
  }

  trackTimestampEdit() {
    trackMixpanelEvent('scope.time.timestamp.edit', {
      layout: this.props.topologyViewMode,
      topologyId: this.props.currentTopology.get('id'),
      parentTopologyId: this.props.currentTopology.get('parentId'),
    });
  }

  trackTimelineClick() {
    trackMixpanelEvent('scope.time.timeline.click', {
      layout: this.props.topologyViewMode,
      topologyId: this.props.currentTopology.get('id'),
      parentTopologyId: this.props.currentTopology.get('parentId'),
    });
  }

  trackTimelinePan() {
    trackMixpanelEvent('scope.time.timeline.pan', {
      layout: this.props.topologyViewMode,
      topologyId: this.props.currentTopology.get('id'),
      parentTopologyId: this.props.currentTopology.get('parentId'),
    });
  }

  render() {
    return (
      <div className={classNames('time-travel', { visible: this.props.showingTimeTravel })}>
        <TimeTravelTimeline
          onTimelinePan={this.handleTimelinePan}
          onTimelinePanEnd={this.handleTimelinePanEnd}
          onInstantJump={this.handleInstantJump}
        />
        <div className="time-travel-timestamp">
          <input
            value={this.state.inputValue}
            onChange={this.handleInputChange}
          /> UTC
        </div>
      </div>
    );
  }
}

function mapStateToProps(state) {
  return {
    showingTimeTravel: state.get('showingTimeTravel'),
    topologyViewMode: state.get('topologyViewMode'),
    currentTopology: state.get('currentTopology'),
    pausedAt: state.get('pausedAt'),
  };
}

export default connect(
  mapStateToProps,
  {
    jumpToTime,
    resumeTime,
    timeTravelStartTransition,
  }
)(TimeTravel);
