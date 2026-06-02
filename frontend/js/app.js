(function(global) {
  'use strict';

  var MAX_CANDLES = 2000;
  var LOADER_TIMEOUT_MS = 15000;
  var PATTERN_DEBOUNCE_MS = 500;

  var candles = [];
  var candlestickSeries = null;
  var chart = null;
  var isInitialized = false;
  var mlAvailable = true;
  var activeTicker = 'BTCUSDT';
  var tickerData = {};
  var countdownInterval = null;
  var currentCloseTime = null;
  var predictionsEnabled = true;
  var initialDataLoaded = false;
  var loaderTimeout = null;
  var patternDebounceTimer = null;

  function init() {
    if (isInitialized) return;

    if (!global.ChartModule) {
      console.error('ChartModule not found.');
      return;
    }

    if (!global.WSClient) {
      console.error('WSClient not found.');
      return;
    }

    chart = global.ChartModule.chart;
    candlestickSeries = global.ChartModule.candlestickSeries;

    if (!chart || !global.ChartModule.updateCandle) {
      console.error('Chart or updateCandle not available.');
      return;
    }

    global.WSClient.onCandle(handleCandle);
    global.WSClient.onSnapshot(handleSnapshot);
    global.WSClient.onStatus(handleStatus);
    global.WSClient.onPrediction(handlePrediction);
    global.WSClient.onTimeframeChange(handleTimeframeChange);

    setupTimeframeSelector();

    tickerData['BTCUSDT'] = { candles: [], smaData: [], predictedSmaData: [] };
    createTab('BTCUSDT');
    var defaultTab = document.querySelector('.tab[data-ticker="BTCUSDT"]');
    if (defaultTab) defaultTab.classList.add('active');

    var addBtn = document.getElementById('add-ticker-btn');
    if (addBtn) addBtn.addEventListener('click', addTicker);

    setupSidebarToggle();
    setupPredictionToggle();
    setupCenterButton();
    startCountdown();
    window.addEventListener('beforeunload', stopCountdown);

    isInitialized = true;
    global.WSClient.connect();
  }

  function setupSidebarToggle() {
    var toggle = document.getElementById('sidebar-toggle');
    var sidebar = document.getElementById('sidebar');
    var mainContent = document.getElementById('main-content');
    if (!toggle || !sidebar) return;

    toggle.addEventListener('click', function() {
      sidebar.classList.toggle('open');
    });

    // Close sidebar when clicking main content on mobile
    if (mainContent) {
      mainContent.addEventListener('click', function() {
        if (sidebar.classList.contains('open')) {
          sidebar.classList.remove('open');
        }
      });
    }
  }

  function setupPredictionToggle() {
    var toggle = document.getElementById('prediction-toggle');
    if (!toggle) return;

    toggle.addEventListener('change', function() {
      predictionsEnabled = toggle.checked;
      if (!predictionsEnabled && global.ChartModule) {
        if (global.ChartModule.setPatternPredictionVisible) {
          global.ChartModule.setPatternPredictionVisible(false);
        }
        clearPredictionsFromChart();
      }
    });
  }

  function setupCenterButton() {
    var btn = document.getElementById('center-chart-btn');
    if (!btn) return;
    btn.addEventListener('click', function() {
      if (global.ChartModule && global.ChartModule.centerLastCandle) {
        global.ChartModule.centerLastCandle();
      }
    });
  }

  function startCountdown() {
    if (countdownInterval) clearInterval(countdownInterval);
    countdownInterval = setInterval(updateCountdownDisplay, 250);
  }

  function stopCountdown() {
    if (countdownInterval) {
      clearInterval(countdownInterval);
      countdownInterval = null;
    }
  }

  var cachedTimerEl = null;
  var cachedTimeframeEl = null;

  function updateCountdownDisplay() {
    if (!cachedTimerEl) cachedTimerEl = document.getElementById('countdown-timer');
    if (!cachedTimeframeEl) cachedTimeframeEl = document.getElementById('countdown-timeframe');
    if (!cachedTimerEl) return;

    var tf = getCurrentTimeframe();
    if (cachedTimeframeEl) cachedTimeframeEl.textContent = tf;

    if (!currentCloseTime) {
      cachedTimerEl.textContent = '--:--';
      return;
    }

    var remaining = currentCloseTime * 1000 - Date.now();
    if (remaining <= 0) {
      cachedTimerEl.textContent = tf === '1s' ? '00' : '00:00';
      return;
    }

    var totalSeconds = Math.floor(remaining / 1000);
    var minutes = Math.floor(totalSeconds / 60);
    var seconds = totalSeconds % 60;

    if (tf === '1s') {
      cachedTimerEl.textContent = String(totalSeconds).padStart(2, '0');
    } else {
      cachedTimerEl.textContent = String(minutes).padStart(2, '0') + ':' + String(seconds).padStart(2, '0');
    }
  }

  function createTab(ticker) {
    var container = document.getElementById('tabs-container');
    if (!container) return;
    if (document.querySelector('.tab[data-ticker="' + ticker + '"]')) return;

    var tab = document.createElement('div');
    tab.className = 'tab';
    tab.dataset.ticker = ticker;

    var name = document.createElement('span');
    name.className = 'tab-name';
    name.textContent = ticker;

    var closeBtn = document.createElement('button');
    closeBtn.className = 'tab-close';
    closeBtn.textContent = '\u00D7';
    closeBtn.addEventListener('click', function(e) {
      e.stopPropagation();
      closeTab(ticker);
    });

    tab.appendChild(name);
    tab.appendChild(closeBtn);
    tab.addEventListener('click', function() { switchTab(ticker); });
    container.appendChild(tab);
  }

  function switchTab(ticker) {
    if (activeTicker === ticker) return;

    saveTickerState(activeTicker);

    document.querySelectorAll('.tab').forEach(function(t) {
      t.classList.remove('active');
    });
    var tab = document.querySelector('.tab[data-ticker="' + ticker + '"]');
    if (tab) tab.classList.add('active');

    activeTicker = ticker;
    currentCloseTime = null;

    restoreTickerState(ticker);
  }

  function closeTab(ticker) {
    var tabs = document.querySelectorAll('.tab');
    if (tabs.length <= 1) return;

    if (global.WSClient && global.WSClient.isConnected) {
      global.WSClient.sendTickerUnsubscribe(ticker);
    }

    var tabEl = document.querySelector('.tab[data-ticker="' + ticker + '"]');
    if (tabEl) tabEl.remove();

    if (activeTicker === ticker) {
      var remainingTab = document.querySelector('.tab');
      if (remainingTab) switchTab(remainingTab.dataset.ticker);
    }

    delete tickerData[ticker];
  }

  function addTicker() {
    var ticker = prompt('Enter ticker symbol (e.g., ETHUSDT):');
    if (!ticker) return;

    var symbol = ticker.toUpperCase().replace(/[^A-Z0-9]/g, '');
    if (!symbol || tickerData[symbol]) {
      alert('Invalid or already open ticker');
      return;
    }

    if (global.WSClient && global.WSClient.isConnected) {
      global.WSClient.sendTickerSubscribe(symbol, getCurrentTimeframe());
    }

    tickerData[symbol] = { candles: [], smaData: [], predictedSmaData: [] };
    createTab(symbol);
    switchTab(symbol);
  }

  function saveTickerState(ticker) {
    if (!tickerData[ticker]) tickerData[ticker] = { candles: [], smaData: [], predictedSmaData: [] };
    tickerData[ticker].candles = candles.slice();
    // Save SMA data from chart module
    if (global.ChartModule) {
      tickerData[ticker].smaData = global.ChartModule.smaData ? global.ChartModule.smaData.slice() : [];
      tickerData[ticker].predictedSmaData = global.ChartModule.predictedSmaData ? global.ChartModule.predictedSmaData.slice() : [];
    }
  }

  function restoreTickerState(ticker) {
    if (!tickerData[ticker]) tickerData[ticker] = { candles: [], smaData: [], predictedSmaData: [] };
    candles = tickerData[ticker].candles.slice();

    if (global.ChartModule) {
      global.ChartModule.clearChart();
      if (candles.length > 0) {
        global.ChartModule.setCandles(candles);
      }
    }

    initialDataLoaded = candles.length >= 10;
    if (initialDataLoaded && chart && chart.timeScale) {
      chart.timeScale().fitContent();
      setTimeout(function() {
        if (global.ChartModule && global.ChartModule.centerLastCandle) {
          global.ChartModule.centerLastCandle();
        }
      }, 150);
    }
  }

  function getCurrentTimeframe() {
    var sel = document.getElementById('timeframe-selector');
    return sel ? sel.value : '1s';
  }

  function setupTimeframeSelector() {
    var selector = document.getElementById('timeframe-selector');
    if (!selector) return;

    selector.addEventListener('change', function(event) {
      var timeframe = event.target.value;
      currentCloseTime = null;
      updateCountdownDisplay();
      if (global.WSClient && global.WSClient.isConnected) {
        global.WSClient.sendTimeframeChange(timeframe, activeTicker);
        showLoader();
        saveTickerState(activeTicker);
        candles = [];
        initialDataLoaded = false;
        if (global.ChartModule && global.ChartModule.clearChart) {
          global.ChartModule.clearChart();
        }
      }
    });
  }

  function handleTimeframeChange(message) {
    if (!message) return;

    hideLoader();
    if (message.status === 'ok') {
      updateStatusBar('Connected (' + message.timeframe + ')', 'status-connected');
    } else {
      updateStatusBar('Timeframe error: ' + (message.message || 'Unknown'), 'status-error');
    }
  }

  function parseCloseTime(closeTime) {
    if (typeof closeTime === 'number') {
      return closeTime > 1e12 ? Math.floor(closeTime / 1000) : closeTime;
    }
    if (typeof closeTime === 'string') {
      var date = new Date(closeTime);
      return Math.floor(date.getTime() / 1000);
    }
    return Math.floor(Date.now() / 1000);
  }

  function handleCandle(candle) {
    if (!candle) return;

    if (candle.symbol && candle.symbol !== activeTicker) {
      if (!tickerData[candle.symbol]) {
        tickerData[candle.symbol] = { candles: [], smaData: [], predictedSmaData: [] };
      }
      return;
    }

    var tvCandle = {
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

    currentCloseTime = tvCandle.time;

    if (!global.ChartModule || !global.ChartModule.updateCandle) return;

    var lastCandle = candles.length > 0 ? candles[candles.length - 1] : null;

    if (lastCandle && lastCandle.time === tvCandle.time) {
      candles[candles.length - 1] = tvCandle;
      global.ChartModule.updateCandle(tvCandle);
    } else {
      candles.push(tvCandle);

      while (candles.length > MAX_CANDLES) {
        candles.shift();
      }

      global.ChartModule.updateCandle(tvCandle);

      if (!initialDataLoaded && candles.length >= 10) {
        initialDataLoaded = true;
        if (chart && chart.timeScale) {
          chart.timeScale().fitContent();
          setTimeout(function() {
            if (global.ChartModule && global.ChartModule.centerLastCandle) {
              global.ChartModule.centerLastCandle();
            }
          }, 150);
        }
      }

      // Debounced pattern analysis
      if (predictionsEnabled && global.PatternEngine && candles.length >= 2 && global.ChartModule && global.ChartModule.updatePatternPrediction) {
        if (patternDebounceTimer) clearTimeout(patternDebounceTimer);
        patternDebounceTimer = setTimeout(function() {
          try {
            var prediction = global.PatternEngine.analyze(candles);
            global.ChartModule.updatePatternPrediction(candles, prediction);
          } catch (err) {
            console.error('Pattern analysis error:', err);
          }
        }, PATTERN_DEBOUNCE_MS);
      }
    }
  }

  function handleSnapshot(rawCandles) {
    if (!rawCandles || !rawCandles.length) return;

    var tvCandles = [];
    for (var i = 0; i < rawCandles.length; i++) {
      var c = rawCandles[i];
      var tvCandle = {
        time: parseCloseTime(c.close_time),
        open: parseFloat(c.open),
        high: parseFloat(c.high),
        low: parseFloat(c.low),
        close: parseFloat(c.close),
      };
      if (!isNaN(tvCandle.time) && !isNaN(tvCandle.open)) {
        tvCandles.push(tvCandle);
      }
    }

    if (tvCandles.length === 0) return;

    // Keep only the last MAX_CANDLES
    if (tvCandles.length > MAX_CANDLES) {
      tvCandles = tvCandles.slice(tvCandles.length - MAX_CANDLES);
    }

    candles = tvCandles;
    currentCloseTime = candles[candles.length - 1].time;

    if (global.ChartModule && global.ChartModule.setCandles) {
      global.ChartModule.setCandles(candles);
    }

    initialDataLoaded = true;
    hideLoader();

    if (chart && chart.timeScale) {
      chart.timeScale().fitContent();
      setTimeout(function() {
        if (global.ChartModule && global.ChartModule.centerLastCandle) {
          global.ChartModule.centerLastCandle();
        }
      }, 150);
    }
  }

  function updateStatusBar(text, statusClass) {
    var statusBar = document.getElementById('status-bar');
    var statusText = document.getElementById('status-text');
    if (!statusBar || !statusText) return;
    statusText.textContent = text;
    statusBar.className = statusClass;
  }

  function hideLoader() {
    if (loaderTimeout) {
      clearTimeout(loaderTimeout);
      loaderTimeout = null;
    }
    var loader = document.getElementById('loader-overlay');
    if (loader) loader.style.display = 'none';
  }

  function showLoader() {
    var loader = document.getElementById('loader-overlay');
    if (loader) loader.style.display = 'flex';

    // Auto-hide loader after timeout to prevent frozen UI
    if (loaderTimeout) clearTimeout(loaderTimeout);
    loaderTimeout = setTimeout(function() {
      hideLoader();
      updateStatusBar('Connection timeout', 'status-error');
    }, LOADER_TIMEOUT_MS);
  }

  function clearPredictionsFromChart() {
    if (!global.ChartModule) return;
    if (global.ChartModule.predictedSmaSeries) {
      global.ChartModule.predictedSmaSeries.setData([]);
      global.ChartModule.predictedSmaSeries.applyOptions({ visible: false });
    }
    if (global.ChartModule.predictedSmaData) {
      global.ChartModule.predictedSmaData.length = 0;
    }
    if (global.ChartModule.predictedSeries) {
      global.ChartModule.predictedSeries.setData([]);
    }
    if (global.ChartModule.predictedCandles) {
      global.ChartModule.predictedCandles.length = 0;
    }
  }

  function restorePredictionsToChart() {
    if (!global.ChartModule) return;
    if (global.ChartModule.predictedSmaSeries && global.ChartModule.predictedSmaData) {
      global.ChartModule.predictedSmaSeries.setData(global.ChartModule.predictedSmaData);
      global.ChartModule.predictedSmaSeries.applyOptions({ visible: true });
    }
  }

  function handleStatus(message) {
    if (!message || typeof message !== 'object') return;

    var status = message.status;

    switch (status) {
      case 'connected':
        if (!mlAvailable) {
          mlAvailable = true;
          restorePredictionsToChart();
        }
        hideLoader();
        updateStatusBar('Connected', 'status-connected');
        break;
      case 'collecting data':
        if (!mlAvailable) {
          mlAvailable = true;
          restorePredictionsToChart();
        }
        hideLoader();
        updateStatusBar('Collecting data...', 'status-collecting');
        break;
      case 'reconnecting':
        updateStatusBar('Reconnecting...', 'status-reconnecting');
        break;
      case 'disconnected':
        updateStatusBar('Disconnected', 'status-disconnected');
        break;
      case 'error':
        updateStatusBar('Connection error', 'status-error');
        break;
      case 'ml_unavailable':
      case 'ML unavailable':
        if (mlAvailable) {
          mlAvailable = false;
          clearPredictionsFromChart();
        }
        updateStatusBar('Prediction unavailable', 'status-ml-unavailable');
        break;
      case 'ml_error':
      case 'ML error':
        if (mlAvailable) {
          mlAvailable = false;
          clearPredictionsFromChart();
        }
        updateStatusBar('Prediction error', 'status-ml-error');
        break;
      default:
        if (!mlAvailable && status !== 'ml_unavailable' && status !== 'ML unavailable' && status !== 'ml_error' && status !== 'ML error') {
          mlAvailable = true;
          restorePredictionsToChart();
        }
        if (status && status.indexOf && status.indexOf('collecting') !== -1) {
          updateStatusBar(status, 'status-collecting');
        }
    }
  }

  function updateConfidenceDisplay(confidence, direction) {
    var el = document.getElementById('prediction-confidence');
    if (!el) return;
    if (typeof confidence === 'number' && !isNaN(confidence)) {
      var pct = Math.round(confidence * 100);
      el.textContent = direction + ' ' + pct + '%';
      el.style.display = 'block';
    } else {
      el.style.display = 'none';
    }
  }

  function handlePrediction(prediction) {
    if (!prediction || !mlAvailable || !predictionsEnabled) return;

    var predictedCandle = prediction.predicted_candle;
    if (!predictedCandle) return;

    var time = parseCloseTime(predictedCandle.close_time || predictedCandle.timestamp);
    var predictedMA = parseFloat(prediction.predicted_ma);
    var direction = prediction.direction || 'UNCERTAIN';
    var confidence = parseFloat(prediction.confidence);

    if (isNaN(time)) return;

    if (!isNaN(predictedMA) && global.ChartModule && global.ChartModule.updatePredictedSMA) {
      global.ChartModule.updatePredictedSMA(time, predictedMA, direction);
    }

    var tvCandle = {
      time: time,
      open: parseFloat(predictedCandle.open),
      high: parseFloat(predictedCandle.high),
      low: parseFloat(predictedCandle.low),
      close: parseFloat(predictedCandle.close),
    };

    if (isNaN(tvCandle.open) || isNaN(tvCandle.high) || isNaN(tvCandle.low) || isNaN(tvCandle.close)) {
      return;
    }

    // Validate OHLC consistency
    if (tvCandle.high < tvCandle.low) return;

    if (global.ChartModule && global.ChartModule.updatePredictedCandle) {
      global.ChartModule.updatePredictedCandle(tvCandle, direction);
    }

    updateConfidenceDisplay(confidence, direction);
  }

  var appModule = {
    init: init,
    get candles() { return candles.slice(); },
    get mlAvailable() { return mlAvailable; },
    parseCloseTime: parseCloseTime,
    handleCandle: handleCandle,
    handleStatus: handleStatus,
    handlePrediction: handlePrediction,
    get isInitialized() { return isInitialized; },
    get activeTicker() { return activeTicker; },
    get tickerData() { return tickerData; },
    stopCountdown: stopCountdown,
    startCountdown: startCountdown,
    updateCountdownDisplay: updateCountdownDisplay
  };

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = appModule;
  } else {
    global.app = appModule;
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', init);
    } else {
      init();
    }
  }

})(typeof window !== 'undefined' ? window : global);
