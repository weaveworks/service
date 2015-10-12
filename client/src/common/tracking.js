const pagePrefix = 'scopeService';

export let trackView = function(page) {
  if (window.ga) {
    window.ga('set', 'page', pagePrefix + page);
    window.ga('send', 'pageview');
  } else {
    console.log('trackView', pagePrefix + page);
  }
}
