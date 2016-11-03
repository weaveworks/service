
describe('PromMetricBrowser', () => {
  const PromMetricBrowser = require('../prom-metric-browser');

  describe('getNextNames', () => {
    const getNextNames = PromMetricBrowser.getNextNames;

    it('should render not fail on empty list', () => {
      expect(getNextNames([], '')).toEqual({});
    });

    it('should return first levels', () => {
      const res = getNextNames(['a', 'a_x', 'b'], '');
      expect(res.a).toEqual(['a', 'a_x']);
      expect(res.b).toEqual(['b']);
    });

    it('should not swallow lower levels', () => {
      const res = getNextNames(['a_x_x', 'a_x_y', 'a_x_z'], 'a_x');
      expect(res.y).toEqual(['y']);
      expect(res.z).toEqual(['z']);
    });
  });
});
