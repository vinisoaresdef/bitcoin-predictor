const { test, expect } = require('@playwright/test');
const WebSocket = require('ws');
const fs = require('fs');
const path = require('path');

const TEST_PORT = 8765;
const TEST_WS_URL = `ws://localhost:${TEST_PORT}/ws`;

test.describe('WebSocket Client Tests', () => {
  let mockWSServer;

  test.beforeAll(async () => {
    mockWSServer = new WebSocket.Server({ port: TEST_PORT });
  });

  test.afterAll(async () => {
    await new Promise((resolve) => {
      mockWSServer.clients.forEach((client) => client.terminate());
      mockWSServer.close(resolve);
    });
  });

  test.beforeEach(async ({ page }) => {
    await page.goto('about:blank');
    
    const wsClientPath = path.join(__dirname, '../frontend/js/ws-client.js');
    const wsClientCode = fs.readFileSync(wsClientPath, 'utf8');
    await page.addScriptTag({ content: wsClientCode });
  });

  test.afterEach(async ({ page }) => {
    await page.evaluate(() => {
      if (window.WSClient) {
        window.WSClient.disconnect();
      }
    });
    
    mockWSServer.clients.forEach((client) => client.terminate());
  });

  test('test_parses_kline_message', async ({ page }) => {
    const receivedCandlesPromise = page.evaluate(async (wsUrl) => {
      const candles = [];
      
      window.WSClient.onCandle((candle) => {
        candles.push(candle);
      });
      
      window.WSClient.connect(wsUrl);
      
      return new Promise((resolve) => {
        setTimeout(() => resolve(candles), 500);
      });
    }, TEST_WS_URL);

    await page.waitForTimeout(100);

    await new Promise((resolve) => {
      if (mockWSServer.clients.size > 0) {
        resolve();
      } else {
        const checkInterval = setInterval(() => {
          if (mockWSServer.clients.size > 0) {
            clearInterval(checkInterval);
            resolve();
          }
        }, 50);
        setTimeout(() => {
          clearInterval(checkInterval);
          resolve();
        }, 1000);
      }
    });

    const testCandle = {
      symbol: 'BTCUSDT',
      interval: '1s',
      open: 50000.00,
      high: 50100.00,
      low: 49900.00,
      close: 50050.00,
      volume: 1.5,
      close_time: new Date().toISOString(),
      timestamp: new Date().toISOString()
    };

    mockWSServer.clients.forEach((client) => {
      if (client.readyState === WebSocket.OPEN) {
        client.send(JSON.stringify({
          type: 'kline',
          candle: testCandle
        }));
      }
    });

    const receivedCandles = await receivedCandlesPromise;

    expect(receivedCandles.length).toBeGreaterThan(0);
    expect(receivedCandles[0]).toMatchObject({
      symbol: 'BTCUSDT',
      interval: '1s',
      open: 50000.00,
      high: 50100.00,
      low: 49900.00,
      close: 50050.00,
      volume: 1.5
    });
  });

  test('test_handles_reconnect', async ({ page }) => {
    await page.evaluate(async (wsUrl) => {
      window.testEvents = [];
      
      window.WSClient.onStatus((status) => {
        window.testEvents.push({ type: 'status', status: status.status });
      });
      
      window.WSClient.connect(wsUrl);
    }, TEST_WS_URL);

    await page.waitForTimeout(200);

    mockWSServer.clients.forEach((client) => {
      client.close();
    });

    await page.waitForTimeout(500);

    const overlayVisible = await page.evaluate(() => {
      const overlay = document.getElementById('ws-reconnecting-overlay');
      return overlay && overlay.style.display === 'flex';
    });

    expect(overlayVisible).toBe(true);
  });
});
