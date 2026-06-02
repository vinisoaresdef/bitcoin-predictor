const assert = require('assert');

let mockCandles = [];
const mockPredictedSmaData = [];
let predictedSmaVisible = true;
let predictedSmaDataSet = [];

const wsCallbacks = {
  candle: [],
  status: [],
  prediction: []
};

global.document = {
  readyState: 'complete',
  addEventListener: function() {},
  getElementById: function(id) {
    if (id === 'status-bar') {
      return {
        className: '',
        classList: {
          add: function(c) { global.document._statusBarClass = c; },
          remove: function() {},
          contains: function(c) { return global.document._statusBarClass === c; }
        }
      };
    }
    if (id === 'status-text') {
      return {
        get textContent() { return global.document._statusText || ''; },
        set textContent(val) { global.document._statusText = val; }
      };
    }
    return null;
  },
  _statusBarClass: '',
  _statusText: ''
};

global.ChartModule = {
  chart: {},
  candlestickSeries: {},
  predictedSmaSeries: {
    setData: function(data) { predictedSmaDataSet = data.slice(); },
    applyOptions: function(opts) { if (opts.visible !== undefined) { predictedSmaVisible = opts.visible; } }
  },
  predictedSmaData: mockPredictedSmaData,
  updateCandle: function(c) { mockCandles.push(c); },
  setCandles: function(c) { mockCandles.length = 0; Array.prototype.push.apply(mockCandles, c); },
  updatePredictedSMA: function(time, value, direction) {
    mockPredictedSmaData.push({ time: time, value: value, direction: direction });
  }
};

global.WSClient = {
  connect: function() {},
  onCandle: function(cb) { wsCallbacks.candle.push(cb); },
  onStatus: function(cb) { wsCallbacks.status.push(cb); },
  onPrediction: function(cb) { wsCallbacks.prediction.push(cb); }
};

const app = require('../js/app.js');
app.init();

function resetState() {
  mockCandles.length = 0;
  mockPredictedSmaData.length = 0;
  predictedSmaDataSet.length = 0;
  predictedSmaVisible = true;
  document._statusBarClass = '';
  document._statusText = '';
}

function triggerStatus(status) {
  wsCallbacks.status.forEach(function(cb) {
    cb({ type: 'status', status: status, timestamp: new Date().toISOString() });
  });
}

function triggerPrediction(prediction) {
  wsCallbacks.prediction.forEach(function(cb) {
    cb(prediction);
  });
}

console.log('Running ML Degradation Unit Tests...\n');

resetState();
triggerStatus('ML unavailable');
assert.strictEqual(document._statusText, 'Prediction unavailable', 'Status text should show Prediction unavailable');
assert.strictEqual(document._statusBarClass, 'status-ml-unavailable', 'Status bar class should be status-ml-unavailable');
assert.strictEqual(predictedSmaVisible, false, 'Prediction series should be hidden');
assert.deepStrictEqual(predictedSmaDataSet, [], 'Predictions should be cleared from chart');
console.log('PASS: ML unavailable clears predictions and updates status');

resetState();
triggerStatus('ML unavailable');
triggerPrediction({ predicted_candle: { close_time: Math.floor(Date.now() / 1000) }, predicted_ma: 50000, direction: 'UP' });
assert.strictEqual(mockPredictedSmaData.length, 0, 'Prediction should not be added when ML unavailable');
console.log('PASS: Predictions ignored when ML unavailable');

resetState();
triggerStatus('connected');
triggerPrediction({ predicted_candle: { close_time: Math.floor(Date.now() / 1000) }, predicted_ma: 50000, direction: 'UP' });
assert.strictEqual(mockPredictedSmaData.length, 1, 'Prediction should be added when ML available');
triggerStatus('ML unavailable');
assert.strictEqual(predictedSmaVisible, false, 'Predictions hidden when ML unavailable');
triggerStatus('connected');
assert.strictEqual(predictedSmaVisible, true, 'Predictions visible when ML recovers');
console.log('PASS: Recovery restores predictions');

resetState();
triggerStatus('ML unavailable');
const testCandle = {
  close_time: Math.floor(Date.now() / 1000),
  open: '50000',
  high: '50100',
  low: '49900',
  close: '50050'
};
wsCallbacks.candle.forEach(function(cb) { cb(testCandle); });
assert.strictEqual(mockCandles.length, 1, 'Candle should be processed during ML outage');
console.log('PASS: Real candles continue during ML outage');

resetState();
triggerStatus('connected');
assert.strictEqual(document._statusText, 'Connected', 'Connected status');
assert.strictEqual(document._statusBarClass, 'status-connected', 'Connected class');

triggerStatus('collecting data');
assert.strictEqual(document._statusText, 'Collecting data...', 'Collecting status');
assert.strictEqual(document._statusBarClass, 'status-collecting', 'Collecting class');

triggerStatus('reconnecting');
assert.strictEqual(document._statusText, 'Reconnecting...', 'Reconnecting status');
assert.strictEqual(document._statusBarClass, 'status-reconnecting', 'Reconnecting class');

triggerStatus('disconnected');
assert.strictEqual(document._statusText, 'Disconnected', 'Disconnected status');
assert.strictEqual(document._statusBarClass, 'status-disconnected', 'Disconnected class');

triggerStatus('error');
assert.strictEqual(document._statusText, 'Connection error', 'Error status');
assert.strictEqual(document._statusBarClass, 'status-error', 'Error class');
console.log('PASS: Status transitions work correctly');

console.log('\nAll unit tests passed!');
