import debug from 'debug';
import React from 'react';
import { toQueryString } from './request';

const log = debug('service:tracking');
const error = debug('service:trackingErr');
const trackPrefix = 'weaveCloudService';

export function trackEvent(subject, action, label, value) {
  if (window.ga) {
    window.ga('send', {
      hitType: 'event',
      eventCategory: trackPrefix + subject,
      eventAction: action,
      eventLabel: label,
      eventValue: value
    });
  } else {
    log('trackEvent', subject, action, label, value);
  }
}

export function trackException(msg, fatal) {
  const message = msg.errors && msg.errors.length > 0 ? msg.errors[0].message : msg;
  if (window.ga) {
    window.ga('send', 'exception', {
      exDescription: message,
      exFatal: !!fatal
    });
  } else {
    error(message);
  }
}

export function trackTiming(subject, action, time) {
  const timing = time || window.performance && window.performance.now();
  if (window.ga) {
    window.ga('send', 'timing', subject, action, timing);
  } else {
    log('trackTiming', subject, action, timing);
  }
}

export function trackView(page) {
  if (window.ga) {
    window.ga('set', 'page', trackPrefix + page);
    window.ga('send', 'pageview');
  } else {
    log('trackView', trackPrefix + page);
  }
}

// pardot
export class PardotSignupIFrame extends React.Component {
  render() {
    let url;
    const query = toQueryString({email: this.props.email});

    if (window.pi) {
      const isHTTPS = (document.location.protocol === 'https:');
      const host = (isHTTPS ? 'https://go.pardot.com' : 'http://go.weave.works');
      url = `${host}/l/123932/2015-10-19/3pmpzj?${query}`;
    } else {
      url = `/api/bogus?${query}`;
      log('trackSignup', this.props.email);
    }

    return <iframe src={url} width="1" height="1" style={{opacity: 0}}></iframe>;
  }
}
