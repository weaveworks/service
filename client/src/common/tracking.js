const pagePrefix = 'scopeService';

export let trackEvent = function(subject, action, label, value) {
  if (window.ga) {
    ga('send', {
      hitType: 'event',
      eventCategory: subject,
      eventAction: action,
      eventLabel: label,
      eventValue: value
    });
  } else {
    console.log('trackEvent', subject, action, label, value);
  }
}

export let trackTiming = function(subject, action, time) {
  let timing = time || window.performance && window.performance.now();
  if (window.ga) {
    window.ga('send', 'timing', subject, action, timing);
  } else {
    console.log('trackTiming', subject, action, timing);
  }
}

export let trackView = function(page) {
  if (window.ga) {
    window.ga('set', 'page', pagePrefix + page);
    window.ga('send', 'pageview');
  } else {
    console.log('trackView', pagePrefix + page);
  }
}