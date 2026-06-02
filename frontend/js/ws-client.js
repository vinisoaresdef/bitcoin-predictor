(function(global) {
  'use strict';

  var MAX_RECONNECT_DELAY = 30000;
  var RECONNECT_BASE_DELAY = 1000;

  var ws = null;
  var reconnectAttempts = 0;
  var reconnectTimer = null;
  var isConnecting = false;
  var currentUrl = null;

  var callbacks = {
    candle: [],
    snapshot: [],
    status: [],
    prediction: [],
    indicator: [],
    timeframeChange: [],
    tickerChange: []
  };

  function getDefaultWSUrl() {
    var loc = global.location;
    if (!loc) return 'ws://localhost:8080/ws';
    var protocol = loc.protocol === 'https:' ? 'wss:' : 'ws:';
    return protocol + '//' + loc.host + '/ws';
  }

  function createReconnectOverlay() {
    var overlay = document.getElementById('ws-reconnecting-overlay');
    if (overlay) return overlay;

    overlay = document.createElement('div');
    overlay.id = 'ws-reconnecting-overlay';
    overlay.style.cssText = [
      'position: fixed',
      'top: 0',
      'left: 0',
      'width: 100%',
      'height: 100%',
      'background-color: rgba(0, 0, 0, 0.7)',
      'display: none',
      'justify-content: center',
      'align-items: center',
      'z-index: 9999',
      'color: white',
      'font-family: sans-serif',
      'font-size: 24px'
    ].join(';');

    var message = document.createElement('div');
    message.textContent = 'Reconnecting...';
    message.style.cssText = 'padding: 20px; background: #333; border-radius: 8px;';
    overlay.appendChild(message);

    document.body.appendChild(overlay);
    return overlay;
  }

  function showReconnectOverlay() {
    var overlay = createReconnectOverlay();
    overlay.style.display = 'flex';
  }

  function hideReconnectOverlay() {
    var overlay = document.getElementById('ws-reconnecting-overlay');
    if (overlay) {
      overlay.style.display = 'none';
    }
  }

  function calculateBackoff(attempt) {
    var delay = RECONNECT_BASE_DELAY * Math.pow(2, attempt);
    return Math.min(delay, MAX_RECONNECT_DELAY);
  }

  function emit(eventName, data) {
    var cbs = callbacks[eventName];
    if (cbs) {
      for (var i = 0; i < cbs.length; i++) {
        try {
          cbs[i](data);
        } catch (err) {
          console.error('Error in callback:', err);
        }
      }
    }
  }

  function handleMessage(event) {
    var message;
    try {
      message = JSON.parse(event.data);
    } catch (err) {
      console.error('Failed to parse WebSocket message:', err);
      return;
    }

    if (!message || typeof message !== 'object') return;

    switch (message.type) {
      case 'status':
        emit('status', message);
        break;
      case 'kline':
        if (message.candle) emit('candle', message.candle);
        break;
      case 'snapshot':
        if (message.candles && Array.isArray(message.candles)) {
          emit('snapshot', message.candles);
        }
        break;
      case 'prediction':
        emit('prediction', message);
        break;
      case 'indicator':
        emit('indicator', message);
        break;
      case 'timeframe_change_response':
        emit('timeframeChange', message);
        break;
      case 'ticker_sub':
        emit('tickerChange', message);
        break;
      case 'ticker_list':
        if (message.tickers && Array.isArray(message.tickers)) {
          message.tickers.forEach(function(ticker) {
            emit('tickerChange', { type: 'ticker_sub', ticker: ticker, status: 'ok' });
          });
        }
        break;
      default:
        break;
    }
  }

  function handleOpen() {
    reconnectAttempts = 0;
    hideReconnectOverlay();
    isConnecting = false;
  }

  function handleClose() {
    ws = null;
    isConnecting = false;

    emit('status', {
      type: 'status',
      status: 'disconnected',
      timestamp: new Date().toISOString()
    });

    scheduleReconnect();
  }

  function handleError(error) {
    console.error('WebSocket error:', error);
  }

  function scheduleReconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
    }

    showReconnectOverlay();

    var delay = calculateBackoff(reconnectAttempts);
    reconnectAttempts++;

    reconnectTimer = setTimeout(function() {
      if (!ws || ws.readyState === WebSocket.CLOSED) {
        connect(currentUrl);
      }
    }, delay);
  }

  function connect(url) {
    if (isConnecting) return;
    if (ws && ws.readyState === WebSocket.OPEN) return;

    isConnecting = true;
    currentUrl = url || getDefaultWSUrl();

    try {
      ws = new WebSocket(currentUrl);
      ws.onopen = handleOpen;
      ws.onmessage = handleMessage;
      ws.onclose = handleClose;
      ws.onerror = handleError;
    } catch (err) {
      console.error('Failed to create WebSocket connection:', err);
      isConnecting = false;
      scheduleReconnect();
    }
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }

    if (ws) {
      ws.onclose = null;
      ws.close();
      ws = null;
    }

    isConnecting = false;
    reconnectAttempts = 0;
  }

  function safeSend(data) {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      console.warn('Cannot send: WebSocket not connected');
      return false;
    }
    try {
      ws.send(typeof data === 'string' ? data : JSON.stringify(data));
      return true;
    } catch (err) {
      console.error('Failed to send WebSocket message:', err);
      return false;
    }
  }

  function sendTimeframeChange(timeframe, ticker) {
    var message = { type: 'timeframe_change', timeframe: timeframe };
    if (ticker) message.ticker = ticker;
    safeSend(message);
  }

  function sendTickerSubscribe(ticker, timeframe) {
    safeSend({
      type: 'ticker_subscribe',
      ticker: ticker,
      timeframe: timeframe
    });
  }

  function sendTickerUnsubscribe(ticker) {
    safeSend({
      type: 'ticker_unsubscribe',
      ticker: ticker
    });
  }

  var WSClient = {
    connect: connect,
    disconnect: disconnect,
    onCandle: function(cb) { if (typeof cb === 'function') callbacks.candle.push(cb); },
    onSnapshot: function(cb) { if (typeof cb === 'function') callbacks.snapshot.push(cb); },
    onStatus: function(cb) { if (typeof cb === 'function') callbacks.status.push(cb); },
    onPrediction: function(cb) { if (typeof cb === 'function') callbacks.prediction.push(cb); },
    onIndicator: function(cb) { if (typeof cb === 'function') callbacks.indicator.push(cb); },
    onTimeframeChange: function(cb) { if (typeof cb === 'function') callbacks.timeframeChange.push(cb); },
    onTickerChange: function(cb) { if (typeof cb === 'function') callbacks.tickerChange.push(cb); },
    sendTimeframeChange: sendTimeframeChange,
    sendTickerSubscribe: sendTickerSubscribe,
    sendTickerUnsubscribe: sendTickerUnsubscribe,
    get isConnected() {
      return ws && ws.readyState === WebSocket.OPEN;
    },
    RECONNECT_BASE_DELAY: RECONNECT_BASE_DELAY,
    MAX_RECONNECT_DELAY: MAX_RECONNECT_DELAY
  };

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = WSClient;
  } else {
    global.WSClient = WSClient;
  }

})(typeof window !== 'undefined' ? window : global);
