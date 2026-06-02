const assert = require('assert');

global.document = {
  getElementById: function(id) {
    if (id === 'chart-container') {
      return {
        clientWidth: 800,
        clientHeight: 600,
      };
    }
    return null;
  },
};

global.window = {
  addEventListener: function() {},
};

const mockSeries = {
  update: function(point) { this.data.push(point); },
  setData: function(data) { this.data = data.slice(); },
  applyOptions: function(opts) { Object.assign(this.options, opts); },
  options: {},
  data: [],
};

const mockChart = {
  addSeries: function(_, opts) { return { ...mockSeries, data: [], options: opts || {} }; },
  applyOptions: function() {},
};

global.LightweightCharts = {
  createChart: function() { return mockChart; },
  CandlestickSeries: 'CandlestickSeries',
  LineSeries: 'LineSeries',
  CrosshairMode: { Normal: 0 },
  LineStyle: { Dashed: 1, Dotted: 2 },
};

const ChartModule = require('../js/chart.js');

let testCount = 0;
let passCount = 0;
let failCount = 0;

function runTest(name, fn) {
  testCount++;
  try {
    fn();
    passCount++;
    console.log('  PASS: ' + name);
  } catch (err) {
    failCount++;
    console.log('  FAIL: ' + name);
    console.log('    ' + err.message);
  }
}

console.log('\nDual Candlestick Series Tests');

runTest('realSeries exists with correct colors', function() {
  assert.ok(ChartModule.realSeries, 'realSeries should exist');
  assert.strictEqual(ChartModule.realSeries.options.upColor, '#26a69a', 'real upColor');
  assert.strictEqual(ChartModule.realSeries.options.downColor, '#ef5350', 'real downColor');
  assert.strictEqual(ChartModule.realSeries.options.borderVisible, true, 'real borderVisible');
});

runTest('predictedSeries exists with RGBA colors', function() {
  assert.ok(ChartModule.predictedSeries, 'predictedSeries should exist');
  assert.ok(ChartModule.predictedSeries.options.upColor.includes('rgba'), 'predicted upColor should be rgba');
  assert.ok(ChartModule.predictedSeries.options.downColor.includes('rgba'), 'predicted downColor should be rgba');
  assert.ok(ChartModule.predictedSeries.options.upColor.includes('0.6'), 'predicted upColor alpha 0.6');
});

runTest('candlestickSeries alias points to realSeries', function() {
  assert.strictEqual(ChartModule.candlestickSeries, ChartModule.realSeries, 'candlestickSeries alias');
});

runTest('updatePredictedCandle sets UP colors', function() {
  ChartModule.updatePredictedCandle(
    { time: 1609459200, open: 50000, high: 51000, low: 49000, close: 50500 },
    'UP'
  );
  assert.strictEqual(ChartModule.predictedSeries.options.upColor, 'rgba(38, 166, 154, 0.6)', 'UP upColor');
  assert.strictEqual(ChartModule.predictedSeries.options.downColor, 'rgba(38, 166, 154, 0.6)', 'UP downColor');
});

runTest('updatePredictedCandle sets DOWN colors', function() {
  ChartModule.updatePredictedCandle(
    { time: 1609459201, open: 50000, high: 51000, low: 49000, close: 49500 },
    'DOWN'
  );
  assert.strictEqual(ChartModule.predictedSeries.options.upColor, 'rgba(239, 83, 80, 0.6)', 'DOWN upColor');
  assert.strictEqual(ChartModule.predictedSeries.options.downColor, 'rgba(239, 83, 80, 0.6)', 'DOWN downColor');
});

runTest('updatePredictedCandle sets UNCERTAIN colors', function() {
  ChartModule.updatePredictedCandle(
    { time: 1609459202, open: 50000, high: 51000, low: 49000, close: 50000 },
    'UNCERTAIN'
  );
  assert.strictEqual(ChartModule.predictedSeries.options.upColor, 'rgba(128, 128, 128, 0.3)', 'UNCERTAIN upColor');
  assert.strictEqual(ChartModule.predictedSeries.options.downColor, 'rgba(128, 128, 128, 0.3)', 'UNCERTAIN downColor');
});

runTest('updatePredictedCandle defaults to UNCERTAIN for unknown direction', function() {
  ChartModule.updatePredictedCandle(
    { time: 1609459203, open: 50000, high: 51000, low: 49000, close: 50000 },
    'UNKNOWN'
  );
  assert.strictEqual(ChartModule.predictedSeries.options.upColor, 'rgba(128, 128, 128, 0.3)', 'UNKNOWN upColor');
});

runTest('updatePredictedCandle creates WhitespaceData gap when real candles exist', function() {
  ChartModule.setCandles([
    { time: 1609459100, open: 50000, high: 50100, low: 49900, close: 50050 },
  ]);
  ChartModule.updatePredictedCandle(
    { time: 1609459204, open: 50050, high: 50200, low: 50000, close: 50150 },
    'UP'
  );
  const data = ChartModule.predictedCandles;
  assert.strictEqual(data.length, 2, 'should have 2 points (gap + candle)');
  assert.strictEqual(data[0].time, 1609459100, 'gap at last real candle time');
  assert.strictEqual(data[0].hasOwnProperty('open'), false, 'gap has no open');
  assert.strictEqual(data[0].hasOwnProperty('close'), false, 'gap has no close');
  assert.strictEqual(data[1].time, 1609459204, 'predicted candle time');
  assert.strictEqual(data[1].close, 50150, 'predicted candle close');
});

console.log('\n-------------------');
console.log('Total: ' + testCount);
console.log('Passed: ' + passCount);
console.log('Failed: ' + failCount);

if (failCount > 0) {
  process.exit(1);
}
