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
  options: {},
  data: [],
};

const mockChart = {
  addSeries: function(type, opts) {
    const series = {
      update: mockSeries.update,
      setData: mockSeries.setData,
      data: [],
      _options: Object.assign({}, opts),
      options: function() { return this._options; },
      applyOptions: function(newOpts) { Object.assign(this._options, newOpts); },
    };
    return series;
  },
  applyOptions: function() {},
};

global.LightweightCharts = {
  createChart: function() { return mockChart; },
  CandlestickSeries: 'CandlestickSeries',
  LineSeries: 'LineSeries',
  CrosshairMode: { Normal: 0 },
  LineStyle: { Solid: 0, Dotted: 1, Dashed: 2, LargeDashed: 3, SparseDotted: 4 },
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

console.log('\nPredicted SMA Series');

runTest('predictedSmaSeries exists with dotted line style', function() {
  assert.ok(ChartModule.predictedSmaSeries, 'predictedSmaSeries should exist');
  assert.strictEqual(
    ChartModule.predictedSmaSeries.options().lineStyle,
    1,
    'predictedSmaSeries should have dotted line style (1)'
  );
});

runTest('predictedSmaSeries has orange color and 1px width', function() {
  assert.strictEqual(
    ChartModule.predictedSmaSeries.options().color,
    'rgba(255, 152, 0, 0.5)',
    'predictedSmaSeries should have orange color'
  );
  assert.strictEqual(
    ChartModule.predictedSmaSeries.options().lineWidth,
    1,
    'predictedSmaSeries should have 1px line width'
  );
});

runTest('predictedSmaSeries is initially hidden', function() {
  assert.strictEqual(
    ChartModule.predictedSmaSeries.options().visible,
    false,
    'predictedSmaSeries should be initially invisible'
  );
});

runTest('updatePredictedSMA updates series with correct value', function() {
  const time = 1609459200;
  const value = 51000.5;
  ChartModule.updatePredictedSMA(time, value, 'UP');

  const lastPoint = ChartModule.predictedSmaData[ChartModule.predictedSmaData.length - 1];
  assert.strictEqual(lastPoint.time, time, 'Time should match');
  assert.strictEqual(lastPoint.value, value, 'Value should match');
});

runTest('updatePredictedSMA sets visible to true', function() {
  ChartModule.updatePredictedSMA(1609459201, 52000, 'UP');
  assert.strictEqual(
    ChartModule.predictedSmaSeries.options().visible,
    true,
    'predictedSmaSeries should be visible after update'
  );
});

runTest('updatePredictedSMA maintains orange color regardless of direction', function() {
  ChartModule.updatePredictedSMA(1609459202, 50000, 'UP');
  assert.strictEqual(
    ChartModule.predictedSmaSeries.options().color,
    'rgba(255, 152, 0, 0.5)',
    'Color should remain orange for UP direction'
  );

  ChartModule.updatePredictedSMA(1609459203, 50000, 'DOWN');
  assert.strictEqual(
    ChartModule.predictedSmaSeries.options().color,
    'rgba(255, 152, 0, 0.5)',
    'Color should remain orange for DOWN direction'
  );

  ChartModule.updatePredictedSMA(1609459204, 50000, 'UNCERTAIN');
  assert.strictEqual(
    ChartModule.predictedSmaSeries.options().color,
    'rgba(255, 152, 0, 0.5)',
    'Color should remain orange for UNCERTAIN direction'
  );
});

runTest('updatePredictedSMA warns on invalid time', function() {
  let warned = false;
  const originalWarn = console.warn;
  console.warn = function(msg) {
    if (msg.includes('Invalid predicted SMA time')) warned = true;
  };

  ChartModule.updatePredictedSMA(NaN, 50000, 'UP');
  assert.strictEqual(warned, true, 'Should warn about invalid time');
  console.warn = originalWarn;
});

runTest('updatePredictedSMA warns on invalid value', function() {
  let warned = false;
  const originalWarn = console.warn;
  console.warn = function(msg) {
    if (msg.includes('Invalid predicted SMA value')) warned = true;
  };

  ChartModule.updatePredictedSMA(1609459205, NaN, 'UP');
  assert.strictEqual(warned, true, 'Should warn about invalid value');
  console.warn = originalWarn;
});

console.log('\n-------------------');
console.log('Total: ' + testCount);
console.log('Passed: ' + passCount);
console.log('Failed: ' + failCount);

if (failCount > 0) {
  process.exit(1);
}
