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

test.describe('Predicted SMA Line Rendering', () => {
    test('test_predicted_sma_line_exists_and_is_dotted', async ({ page }) => {
        const pageErrors = [];
        page.on('pageerror', err => pageErrors.push(err.message));

        await page.goto('/');
        await page.waitForTimeout(500);

        const predictedSmaInfo = await page.evaluate(() => {
            const series = window.ChartModule.predictedSmaSeries;
            return {
                seriesExists: !!series,
                lineStyle: series ? series.options().lineStyle : null,
            };
        });

        expect(predictedSmaInfo.seriesExists).toBe(true);
        expect(predictedSmaInfo.lineStyle).toBe(1);
        expect(pageErrors).toEqual([]);
    });

    test('test_predicted_sma_different_style', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        const opts = await page.evaluate(() => {
            return window.ChartModule.predictedSmaSeries.options();
        });

        expect(opts.color).toBe('rgba(255, 152, 0, 0.5)');
        expect(opts.lineWidth).toBe(1);
        expect(opts.lineStyle).toBe(1);
    });

    test('test_predicted_sma_visible', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        const baseTime = Math.floor(Date.now() / 1000);

        // Add 20 candles so SMA is calculated
        for (let i = 0; i < 20; i++) {
            await page.evaluate((candle) => {
                window.ChartModule.updateCandle(candle);
            }, {
                time: baseTime + i,
                open: 50000 + i * 100,
                high: 50100 + i * 100,
                low: 49900 + i * 100,
                close: 50050 + i * 100,
            });
        }

        // Before prediction, predicted SMA should be empty/invisible
        let info = await page.evaluate(() => {
            return {
                dataLength: window.ChartModule.predictedSmaData.length,
                visible: window.ChartModule.predictedSmaSeries.options().visible,
            };
        });
        expect(info.dataLength).toBe(0);
        expect(info.visible).toBe(false);

        // Trigger prediction
        await page.evaluate((bt) => {
            window.ChartModule.updatePredictedSMA(bt + 30, 52000, 'UP');
        }, baseTime);

        // After prediction, should have continuity + predicted points
        info = await page.evaluate(() => {
            return {
                dataLength: window.ChartModule.predictedSmaData.length,
                visible: window.ChartModule.predictedSmaSeries.options().visible,
                lastPoint: window.ChartModule.predictedSmaData[window.ChartModule.predictedSmaData.length - 1],
            };
        });
        expect(info.dataLength).toBe(2);
        expect(info.visible).toBe(true);
        expect(info.lastPoint.time).toBe(baseTime + 30);
        expect(info.lastPoint.value).toBe(52000);
    });

    test('test_predicted_sma_updates_via_websocket', async ({ page }) => {
        const TEST_PORT = 8767;
        const TEST_WS_URL = `ws://localhost:${TEST_PORT}/ws`;

        const mockWSServer = new WebSocket.Server({ port: TEST_PORT });

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

        const predictionTime = Math.floor(Date.now() / 1000) + 60;
        const testPrediction = {
            type: 'prediction',
            direction: 'UP',
            confidence: 0.85,
            predicted_candle: {
                close_time: predictionTime,
            },
            predicted_ma: 52000.5,
        };

        mockWSServer.clients.forEach((client) => {
            if (client.readyState === WebSocket.OPEN) {
                client.send(JSON.stringify(testPrediction));
            }
        });

        await page.waitForTimeout(300);

        const predictedSmaInfo = await page.evaluate(() => {
            const data = window.ChartModule.predictedSmaData;
            const series = window.ChartModule.predictedSmaSeries;
            return {
                dataLength: data ? data.length : 0,
                lastPoint: data && data.length > 0 ? data[data.length - 1] : null,
                color: series ? series.options().color : null,
                visible: series ? series.options().visible : null,
            };
        });

        expect(predictedSmaInfo.dataLength).toBeGreaterThan(0);
        expect(predictedSmaInfo.lastPoint.value).toBe(52000.5);
        expect(predictedSmaInfo.color).toBe('rgba(38, 166, 154, 0.6)');
        expect(predictedSmaInfo.visible).toBe(true);

        await new Promise((resolve) => {
            mockWSServer.clients.forEach((client) => client.terminate());
            mockWSServer.close(resolve);
        });
    });
});

