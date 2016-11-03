
// post instance to katacoda for token use
export function katacodaPostInstance(instance) {
  if (window.parent !== window) {
    window.parent.postMessage(instance, 'https://www.katacoda.com');
  }
}
