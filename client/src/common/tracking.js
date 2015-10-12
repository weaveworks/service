const pagePrefix = 'scopeService';

export let pageView = function(page) {
  if (window.ga) {
    window.ga('set', 'page', pagePrefix + page);
    window.ga('send', 'pageview');
  } else {
    console.log('pageView', pagePrefix + page);
  }
}
