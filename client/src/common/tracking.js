import debug from 'debug';

import { postForm } from './request';

const log = debug('service:tracking');
const error = debug('service:trackingErr');
const trackPrefix = 'scopeService';

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
  if (window.ga) {
    window.ga('send', 'exception', {
      exDescription: msg,
      exFatal: !!fatal
    });
  } else {
    error(msg);
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
export function trackSignup(email) {
  if (window.pi) {
    const isHTTPS = (document.location.protocol === 'https:');
    const url = (isHTTPS ? 'https://go.pardot.com' : 'http://go.weave.works')
      + '/l/123932/2015-10-19/3pmpzj';
    postForm(url, {email: email})
      .then(function handleSuccess() {
      }, trackException);
  } else {
    log('trackSignup', email);
  }
}
