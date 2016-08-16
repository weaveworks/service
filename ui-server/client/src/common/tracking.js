import debug from 'debug';
import { postData } from './request';

const log = debug('service:tracking');
const error = debug('service:trackingErr');
const trackPrefix = 'weaveCloudService';

let lastView = null;
let lastViewAt = null;

export function generateSessionCookie() {
  return '_weaveclientid=xxxxxxxx'.replace(/x/g, () => (Math.random() * 16 | 0).toString(16));
}

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

export function trackView(page, state) {
  const view = page.toLowerCase();
  if (window.ga) {
    window.ga('set', 'page', trackPrefix + view);
    window.ga('send', 'pageview');
  } else {
    log('trackView', trackPrefix + view, lastView);
  }

  // track page transitions
  let lastViewTimeSpent;
  if (lastViewAt) {
    // navigated to this page
    lastViewTimeSpent = (new Date()) - lastViewAt;
  } else {
    // landed on this page
    lastViewTimeSpent = (window.performance && window.performance.now()) || 0;
  }
  postData('/api/analytics/view', {
    state,
    view,
    lastView,
    lastViewTimeSpent
  }).catch(() => {});
  lastView = view;
  lastViewAt = new Date();
}
