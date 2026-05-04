const { test, expect } = require('@playwright/test');
const WebSocket = require('ws');

const TEST_PORT = 8766;
const TEST_WS_URL = `ws://localhost:${TEST_PORT}/ws`;

test.describe('TradingView Chart Initialization', () => {
    test('test_chart_canvas_exists', async ({ page }) => {
        const pageErrors = [];
        page.on('pageerror', err => pageErrors.push(err.message));

        await page.goto('/');

        const chartContainer = page.locator('#chart-container');
        await expect(chartContainer).toBeVisible();

        const canvases = chartContainer.locator('canvas');
        await expect(canvases).toHaveCount(7);

        const firstCanvas = canvases.first();
        const canvasBox = await firstCanvas.boundingBox();
        expect(canvasBox.width).toBeGreaterThan(0);
        expect(canvasBox.height).toBeGreaterThan(0);

        await page.waitForTimeout(500);
        expect(pageErrors).toEqual([]);
    });
});

test.describe('Real-time Chart Updates', () => {
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

    test.afterEach(async () => {
        mockWSServer.clients.forEach((client) => client.terminate());
    });

    test('test_updates_chart_on_candle', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        await page.evaluate((wsUrl) => {
            if (window.WSClient) {
                window.WSClient.disconnect();
                window.WSClient.connect(wsUrl);
            }
        }, TEST_WS_URL);

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
            type: 'kline',
            candle: {
                symbol: 'BTCUSDT',
                interval: '1s',
                open: 50000.00,
                high: 50100.00,
                low: 49900.00,
                close: 50050.00,
                volume: 1.5,
                close_time: Date.now(),
                timestamp: new Date().toISOString()
            }
        };

        mockWSServer.clients.forEach((client) => {
            if (client.readyState === WebSocket.OPEN) {
                client.send(JSON.stringify(testCandle));
            }
        });

        await page.waitForTimeout(300);

        const candles = await page.evaluate(() => {
            return window.app && window.app.candles ? window.app.candles : [];
        });

        expect(candles.length).toBeGreaterThan(0);
        expect(candles[0]).toMatchObject({
            open: 50000.00,
            high: 50100.00,
            low: 49900.00,
            close: 50050.00
        });
    });
});

test.describe('SMA Line Rendering', () => {
    test('test_sma_line_visible', async ({ page }) => {
        const pageErrors = [];
        page.on('pageerror', err => pageErrors.push(err.message));

        await page.goto('/');

        const chartContainer = page.locator('#chart-container');
        await expect(chartContainer).toBeVisible();

        const baseTime = Math.floor(Date.now() / 1000) - 30;
        const mockCandles = [];
        for (let i = 0; i < 25; i++) {
            const price = 50000 + i * 100;
            mockCandles.push({
                time: baseTime + i,
                open: price,
                high: price + 50,
                low: price - 50,
                close: price
            });
        }

        await page.evaluate((candles) => {
            candles.forEach(candle => {
                window.ChartModule.updateCandle(candle);
            });
        }, mockCandles);

        await page.waitForTimeout(500);

        const smaInfo = await page.evaluate(() => {
            const series = window.ChartModule.smaSeries;
            const data = window.ChartModule.smaData;
            return {
                seriesExists: !!series,
                dataLength: data ? data.length : 0,
                firstPoint: data && data.length > 0 ? data[0] : null,
                lastPoint: data && data.length > 0 ? data[data.length - 1] : null
            };
        });

        expect(smaInfo.seriesExists).toBe(true);
        expect(smaInfo.dataLength).toBe(6);

        if (smaInfo.firstPoint) {
            expect(smaInfo.firstPoint.time).toBe(baseTime + 19);
            expect(smaInfo.firstPoint.value).toBeCloseTo(50950, 2);
        }

        if (smaInfo.lastPoint) {
            expect(smaInfo.lastPoint.time).toBe(baseTime + 24);
            expect(smaInfo.lastPoint.value).toBeCloseTo(51450, 2);
        }

        expect(pageErrors).toEqual([]);
    });
});

