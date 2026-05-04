(function(global) {
  'use strict';

  const DEFAULT_WS_URL = 'ws://localhost:8080/ws';
  const MAX_RECONNECT_DELAY = 30000;
  const RECONNECT_BASE_DELAY = 1000;

  let ws = null;
  let reconnectAttempts = 0;
  let reconnectTimer = null;
  let isConnecting = false;
  let currentUrl = DEFAULT_WS_URL;

  const callbacks = {
    candle: [],
    status: [],
    prediction: [],
    indicator: []
  };

  function createReconnectOverlay() {
    let overlay = document.getElementById('ws-reconnecting-overlay');
    if (!overlay) {
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
      
      const message = document.createElement('div');
      message.textContent = 'Reconnecting...';
      message.style.cssText = 'padding: 20px; background: #333; border-radius: 8px;';
      overlay.appendChild(message);
      
      document.body.appendChild(overlay);
    }
    return overlay;
  }

  function showReconnectOverlay() {
    const overlay = createReconnectOverlay();
    overlay.style.display = 'flex';
  }

  function hideReconnectOverlay() {
    const overlay = document.getElementById('ws-reconnecting-overlay');
    if (overlay) {
      overlay.style.display = 'none';
    }
  }

  function calculateBackoff(attempt) {
    const delay = RECONNECT_BASE_DELAY * Math.pow(2, attempt);
    return Math.min(delay, MAX_RECONNECT_DELAY);
  }

  function emit(eventName, data) {
    if (callbacks[eventName]) {
      callbacks[eventName].forEach(cb => {
        try {
          cb(data);
        } catch (err) {
          console.error('Error in callback:', err);
        }
      });
    }
  }

  function handleMessage(event) {
    let message;
    try {
      message = JSON.parse(event.data);
    } catch (err) {
      console.error('Failed to parse WebSocket message:', err);
      return;
    }

    if (!message || typeof message !== 'object') {
      return;
    }

    const messageType = message.type;

    switch (messageType) {
      case 'status':
        emit('status', message);
        break;
      case 'kline':
        if (message.candle) {
          emit('candle', message.candle);
        }
        break;
      case 'prediction':
        emit('prediction', message);
        break;
      case 'indicator':
        emit('indicator', message);
        break;
      default:
        console.warn('Unknown message type:', messageType);
    }
  }

  function handleOpen() {
    reconnectAttempts = 0;
    hideReconnectOverlay();
    isConnecting = false;
    
    const reconnectEvent = new CustomEvent('ws-reconnected', {
      detail: { timestamp: Date.now() }
    });
    document.dispatchEvent(reconnectEvent);
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
    
    emit('status', {
      type: 'status',
      status: 'error',
      timestamp: new Date().toISOString()
    });
  }

  function scheduleReconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
    }

    showReconnectOverlay();

    const delay = calculateBackoff(reconnectAttempts);
    reconnectAttempts++;

    reconnectTimer = setTimeout(() => {
      if (!ws || ws.readyState === WebSocket.CLOSED) {
        connect(currentUrl);
      }
    }, delay);
  }

  function connect(url) {
    if (isConnecting) {
      return;
    }

    if (ws && ws.readyState === WebSocket.OPEN) {
      return;
    }

    isConnecting = true;
    currentUrl = url || DEFAULT_WS_URL;

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

  function onCandle(callback) {
    if (typeof callback === 'function') {
      callbacks.candle.push(callback);
    }
  }

  function onStatus(callback) {
    if (typeof callback === 'function') {
      callbacks.status.push(callback);
    }
  }

  function onPrediction(callback) {
    if (typeof callback === 'function') {
      callbacks.prediction.push(callback);
    }
  }

  function onIndicator(callback) {
    if (typeof callback === 'function') {
      callbacks.indicator.push(callback);
    }
  }

  const WSClient = {
    connect: connect,
    disconnect: disconnect,
    onCandle: onCandle,
    onStatus: onStatus,
    onPrediction: onPrediction,
    onIndicator: onIndicator,
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
