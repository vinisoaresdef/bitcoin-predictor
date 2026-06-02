const { test, expect } = require('@playwright/test');
const WebSocket = require('ws');

test.describe('ML Unavailable Graceful Degradation', () => {
  let mockWSServer;
  let testWsUrl;

  test.beforeAll(async () => {
    mockWSServer = new WebSocket.Server({ port: 0 });
    const port = mockWSServer.address().port;
    testWsUrl = `ws://localhost:${port}/ws`;
  });

  test.afterAll(async () => {
    await new Promise((resolve) => {
      mockWSServer.clients.forEach((client) => client.terminate());
      mockWSServer.close(resolve);
    });
  });

  test.afterEach(async () => {
    mockWSServer.clients.forEach((client) => client.terminate());
  });

  async function waitForClient() {
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
  }

  async function sendToAllClients(data) {
    mockWSServer.clients.forEach((client) => {
      if (client.readyState === WebSocket.OPEN) {
        client.send(JSON.stringify(data));
      }
    });
  }

  test('status shows Prediction unavailable when ML service down', async ({ page }) => {
    await page.goto('/');
    await page.waitForTimeout(500);

    await page.evaluate((wsUrl) => {
      if (window.WSClient) {
        window.WSClient.disconnect();
        window.WSClient.connect(wsUrl);
      }
    }, testWsUrl);

    await waitForClient();

    await sendToAllClients({
      type: 'status',
      status: 'ML unavailable',
      timestamp: new Date().toISOString()
    });

    await page.waitForTimeout(300);

    const statusBar = page.locator('#status-bar');
    const statusText = page.locator('#status-text');

    await expect(statusBar).toHaveClass(/status-ml-unavailable/);
    await expect(statusText).toHaveText('Prediction unavailable');
  });

  test('predicted series clears when ML unavailable', async ({ page }) => {
    await page.goto('/');
    await page.waitForTimeout(500);

    await page.evaluate((wsUrl) => {
      if (window.WSClient) {
        window.WSClient.disconnect();
        window.WSClient.connect(wsUrl);
      }
    }, testWsUrl);

    await waitForClient();

    const now = Math.floor(Date.now() / 1000);

    await sendToAllClients({
      type: 'prediction',
      predicted_candle: { close_time: now + 10 },
      predicted_ma: 51000,
      direction: 'UP'
    });

    await page.waitForTimeout(300);

    const predictionCountBefore = await page.evaluate(() => {
      return window.ChartModule && window.ChartModule.predictedSmaData
        ? window.ChartModule.predictedSmaData.length
        : 0;
    });

    expect(predictionCountBefore).toBeGreaterThan(0);

    await sendToAllClients({
      type: 'status',
      status: 'ML unavailable',
      timestamp: new Date().toISOString()
    });

    await page.waitForTimeout(300);

    const predictionCountAfter = await page.evaluate(() => {
      return window.ChartModule && window.ChartModule.predictedSmaData
        ? window.ChartModule.predictedSmaData.length
        : 0;
    });

    expect(predictionCountAfter).toBe(0);
  });

  test('predictions resume automatically when ML recovers', async ({ page }) => {
    await page.goto('/');
    await page.waitForTimeout(500);

    await page.evaluate((wsUrl) => {
      if (window.WSClient) {
        window.WSClient.disconnect();
        window.WSClient.connect(wsUrl);
      }
    }, testWsUrl);

    await waitForClient();

    const now = Math.floor(Date.now() / 1000);

    await sendToAllClients({
      type: 'prediction',
      predicted_candle: { close_time: now + 10 },
      predicted_ma: 51000,
      direction: 'UP'
    });

    await page.waitForTimeout(200);

    await sendToAllClients({
      type: 'status',
      status: 'ML unavailable',
      timestamp: new Date().toISOString()
    });

    await page.waitForTimeout(200);

    await sendToAllClients({
      type: 'status',
      status: 'connected',
      timestamp: new Date().toISOString()
    });

    await page.waitForTimeout(200);

    const isVisible = await page.evaluate(() => {
      const series = window.ChartModule && window.ChartModule.predictedSmaSeries;
      if (!series) return false;
      const opts = series.options ? series.options() : (series._options || {});
      return opts.visible !== false;
    });

    expect(isVisible).toBe(true);
  });

  test('real candles continue updating during ML outage', async ({ page }) => {
    await page.goto('/');
    await page.waitForTimeout(500);

    await page.evaluate((wsUrl) => {
      if (window.WSClient) {
        window.WSClient.disconnect();
        window.WSClient.connect(wsUrl);
      }
    }, testWsUrl);

    await waitForClient();

    const now = Math.floor(Date.now() / 1000);

    await sendToAllClients({
      type: 'status',
      status: 'ML unavailable',
      timestamp: new Date().toISOString()
    });

    await page.waitForTimeout(200);

    const candleCountBefore = await page.evaluate(() => {
      return window.app && window.app.candles ? window.app.candles.length : 0;
    });

    await sendToAllClients({
      type: 'kline',
      candle: {
        symbol: 'BTCUSDT',
        interval: '1s',
        open: 50000.00,
        high: 50100.00,
        low: 49900.00,
        close: 50050.00,
        volume: 1.5,
        close_time: now,
        timestamp: new Date().toISOString()
      }
    });

    await page.waitForTimeout(300);

    const candleCountAfter = await page.evaluate(() => {
      return window.app && window.app.candles ? window.app.candles.length : 0;
    });

    expect(candleCountAfter).toBeGreaterThan(candleCountBefore);
  });

  test('no console errors during ML unavailable state', async ({ page }) => {
    await page.goto('/');
    await page.waitForTimeout(500);

    await page.evaluate((wsUrl) => {
      if (window.WSClient) {
        window.WSClient.disconnect();
        window.WSClient.connect(wsUrl);
      }
    }, testWsUrl);

    await waitForClient();

    const pageErrors = [];
    page.on('pageerror', err => pageErrors.push(err.message));
    page.on('console', msg => {
      if (msg.type() === 'error') {
        pageErrors.push(msg.text());
      }
    });

    await sendToAllClients({
      type: 'status',
      status: 'ML unavailable',
      timestamp: new Date().toISOString()
    });

    await page.waitForTimeout(500);

    const mlRelatedErrors = pageErrors.filter(err =>
      err.includes('ML') ||
      err.includes('prediction') ||
      err.includes('Prediction') ||
      err.includes('unavailable')
    );

    expect(mlRelatedErrors).toEqual([]);
  });

  test('status shows Prediction error on ml_error', async ({ page }) => {
    await page.goto('/');
    await page.waitForTimeout(500);

    await page.evaluate((wsUrl) => {
      if (window.WSClient) {
        window.WSClient.disconnect();
        window.WSClient.connect(wsUrl);
      }
    }, testWsUrl);

    await waitForClient();

    await sendToAllClients({
      type: 'status',
      status: 'ml_error',
      timestamp: new Date().toISOString()
    });

    await page.waitForTimeout(300);

    const statusBar = page.locator('#status-bar');
    const statusText = page.locator('#status-text');

    await expect(statusBar).toHaveClass(/status-ml-error/);
    await expect(statusText).toHaveText('Prediction error');
  });

  test('predicted candles clear when ML unavailable', async ({ page }) => {
    await page.goto('/');
    await page.waitForTimeout(500);

    await page.evaluate((wsUrl) => {
      if (window.WSClient) {
        window.WSClient.disconnect();
        window.WSClient.connect(wsUrl);
      }
    }, testWsUrl);

    await waitForClient();

    const now = Math.floor(Date.now() / 1000);

    await sendToAllClients({
      type: 'prediction',
      predicted_candle: { close_time: now + 10, open: 50000, high: 51000, low: 49000, close: 50500 },
      predicted_ma: 51000,
      direction: 'UP'
    });

    await page.waitForTimeout(300);

    const hasPredictedCandlesBefore = await page.evaluate(() => {
      return window.ChartModule && window.ChartModule.predictedCandles
        ? window.ChartModule.predictedCandles.length > 0
        : false;
    });

    expect(hasPredictedCandlesBefore).toBe(true);

    await sendToAllClients({
      type: 'status',
      status: 'ML unavailable',
      timestamp: new Date().toISOString()
    });

    await page.waitForTimeout(300);

    const hasPredictedCandlesAfter = await page.evaluate(() => {
      return window.ChartModule && window.ChartModule.predictedCandles
        ? window.ChartModule.predictedCandles.length > 0
        : false;
    });

    expect(hasPredictedCandlesAfter).toBe(false);
  });
});
