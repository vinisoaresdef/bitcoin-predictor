(function(global) {
  'use strict';

  const VISIBLE_WINDOW_SECONDS = 60;

  let candles = [];
  let candlestickSeries = null;
  let chart = null;
  let isInitialized = false;

  function init() {
    if (isInitialized) {
      return;
    }

    if (!global.ChartModule) {
      console.error('ChartModule not found. Chart not initialized.');
      return;
    }

    if (!global.WSClient) {
      console.error('WSClient not found. WebSocket client not loaded.');
      return;
    }

    chart = global.ChartModule.chart;
    candlestickSeries = global.ChartModule.candlestickSeries;

    if (!chart || !global.ChartModule.updateCandle) {
      console.error('Chart or updateCandle not available.');
      return;
    }

    global.WSClient.onCandle(handleCandle);

    isInitialized = true;

    global.WSClient.connect();
  }

  function parseCloseTime(closeTime) {
    if (typeof closeTime === 'number') {
      return closeTime > 1e12 ? Math.floor(closeTime / 1000) : closeTime;
    }
    if (typeof closeTime === 'string') {
      const date = new Date(closeTime);
      return Math.floor(date.getTime() / 1000);
    }
    return Math.floor(Date.now() / 1000);
  }

  function handleCandle(candle) {
    if (!candle) {
      console.warn('Cannot handle candle: missing candle');
      return;
    }

    const tvCandle = {
      time: parseCloseTime(candle.close_time),
      open: parseFloat(candle.open),
      high: parseFloat(candle.high),
      low: parseFloat(candle.low),
      close: parseFloat(candle.close),
    };

    if (isNaN(tvCandle.time) || isNaN(tvCandle.open)) {
      console.warn('Invalid candle data:', candle);
      return;
    }

    if (!global.ChartModule || !global.ChartModule.updateCandle) {
      console.warn('ChartModule.updateCandle not available');
      return;
    }

    const lastCandle = candles.length > 0 ? candles[candles.length - 1] : null;

    if (lastCandle && lastCandle.time === tvCandle.time) {
      candles[candles.length - 1] = tvCandle;
      global.ChartModule.updateCandle(tvCandle);
    } else {
      candles.push(tvCandle);

      const cutoffTime = tvCandle.time - VISIBLE_WINDOW_SECONDS;
      const prevLength = candles.length;
      candles = candles.filter(function(c) { return c.time >= cutoffTime; });
      candles.sort(function(a, b) { return a.time - b.time; });

      if (candles.length === 1 || candles.length < prevLength) {
        global.ChartModule.setCandles(candles);
      } else {
        global.ChartModule.updateCandle(tvCandle);
      }
    }
  }

  const app = {
    init: init,
    get candles() { return candles.slice(); },
    parseCloseTime: parseCloseTime,
    handleCandle: handleCandle,
    get isInitialized() { return isInitialized; }
  };

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = app;
  } else {
    global.app = app;
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', init);
    } else {
      init();
    }
  }

})(typeof window !== 'undefined' ? window : global);
