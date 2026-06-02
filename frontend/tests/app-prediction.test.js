const assert = require('assert');

let updatePredictedSMACalls = [];

global.document = {
  readyState: 'complete',
  addEventListener: function() {},
  getElementById: function(id) {
    if (id === 'status-bar') {
      return { className: '', classList: { add: function() {} } };
    }
    if (id === 'status-text') {
      return { textContent: '' };
    }
    return null;
  }
};

global.ChartModule = {
  chart: {},
  candlestickSeries: {},
  updateCandle: function() {},
  setCandles: function() {},
  updatePredictedSMA: function(time, value, direction) {
    updatePredictedSMACalls.push({ time: time, value: value, direction: direction });
  }
};

global.WSClient = {
  connect: function() {},
  onCandle: function() {},
  onStatus: function() {},
  onPrediction: function() {}
};

const app = require('../js/app.js');

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

console.log('\nApp Prediction Handling');

runTest('handlePrediction parses predicted_candle and predicted_ma', function() {
  updatePredictedSMACalls.length = 0;
  const prediction = {
    predicted_candle: { close_time: 1609459200 },
    predicted_ma: 52000.5,
    direction: 'UP'
  };
  app.handlePrediction(prediction);
  assert.strictEqual(updatePredictedSMACalls.length, 1, 'updatePredictedSMA should be called once');
  assert.strictEqual(updatePredictedSMACalls[0].time, 1609459200, 'Time should match');
  assert.strictEqual(updatePredictedSMACalls[0].value, 52000.5, 'Value should match');
  assert.strictEqual(updatePredictedSMACalls[0].direction, 'UP', 'Direction should match');
});

runTest('handlePrediction defaults direction to UNCERTAIN', function() {
  updatePredictedSMACalls.length = 0;
  const prediction = {
    predicted_candle: { close_time: 1609459201 },
    predicted_ma: 51000
  };
  app.handlePrediction(prediction);
  assert.strictEqual(updatePredictedSMACalls[0].direction, 'UNCERTAIN', 'Direction should default to UNCERTAIN');
});

runTest('handlePrediction warns on missing predicted_candle', function() {
  let warned = false;
  const originalWarn = console.warn;
  console.warn = function(msg) {
    if (msg.includes('Prediction missing predicted_candle')) warned = true;
  };
  updatePredictedSMACalls.length = 0;
  app.handlePrediction({ predicted_ma: 50000 });
  assert.strictEqual(warned, true, 'Should warn about missing predicted_candle');
  assert.strictEqual(updatePredictedSMACalls.length, 0, 'Should not call updatePredictedSMA');
  console.warn = originalWarn;
});

runTest('handlePrediction warns on invalid data', function() {
  let warned = false;
  const originalWarn = console.warn;
  console.warn = function(msg) {
    if (msg.includes('Invalid prediction data')) warned = true;
  };
  updatePredictedSMACalls.length = 0;
  app.handlePrediction({ predicted_candle: { close_time: 'invalid' }, predicted_ma: NaN });
  assert.strictEqual(warned, true, 'Should warn about invalid data');
  assert.strictEqual(updatePredictedSMACalls.length, 0, 'Should not call updatePredictedSMA');
  console.warn = originalWarn;
});

runTest('handlePrediction handles string close_time', function() {
  updatePredictedSMACalls.length = 0;
  const prediction = {
    predicted_candle: { close_time: '2021-01-01T00:00:00Z' },
    predicted_ma: 50000,
    direction: 'DOWN'
  };
  app.handlePrediction(prediction);
  assert.strictEqual(updatePredictedSMACalls.length, 1, 'updatePredictedSMA should be called');
  assert.strictEqual(updatePredictedSMACalls[0].direction, 'DOWN', 'Direction should be DOWN');
});

console.log('\n-------------------');
console.log('Total: ' + testCount);
console.log('Passed: ' + passCount);
console.log('Failed: ' + failCount);

if (failCount > 0) {
  process.exit(1);
}
