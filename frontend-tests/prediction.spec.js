const { test, expect } = require('@playwright/test');
const WebSocket = require('ws');

const TEST_PORT = 8770;
const TEST_WS_URL = `ws://localhost:${TEST_PORT}/ws`;

test.describe('Dual Candlestick Series', () => {
    test('both realSeries and predictedSeries exist', async ({ page }) => {
        const pageErrors = [];
        page.on('pageerror', err => pageErrors.push(err.message));

        await page.goto('/');
        await page.waitForTimeout(500);

        const seriesInfo = await page.evaluate(() => {
            return {
                realSeriesExists: !!window.ChartModule.realSeries,
                predictedSeriesExists: !!window.ChartModule.predictedSeries,
            };
        });

        expect(seriesInfo.realSeriesExists).toBe(true);
        expect(seriesInfo.predictedSeriesExists).toBe(true);
        expect(pageErrors).toEqual([]);
    });

    test('predicted series uses RGBA colors with 0.6 alpha', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        const colors = await page.evaluate(() => {
            const opts = window.ChartModule.predictedSeries.options();
            return {
                upColor: opts.upColor,
                downColor: opts.downColor,
                wickUpColor: opts.wickUpColor,
                wickDownColor: opts.wickDownColor,
            };
        });

        expect(colors.upColor).toMatch(/rgba\(.*0\.6\)/);
        expect(colors.downColor).toMatch(/rgba\(.*0\.6\)/);
        expect(colors.wickUpColor).toMatch(/rgba\(.*0\.6\)/);
        expect(colors.wickDownColor).toMatch(/rgba\(.*0\.6\)/);
    });

    test('real series preserves default TradingView colors', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        const colors = await page.evaluate(() => {
            const opts = window.ChartModule.realSeries.options();
            return {
                upColor: opts.upColor,
                downColor: opts.downColor,
            };
        });

        expect(colors.upColor).toBe('#26a69a');
        expect(colors.downColor).toBe('#ef5350');
    });

    test('test_predicted_candle_has_different_color', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        const colors = await page.evaluate(() => {
            const realOpts = window.ChartModule.realSeries.options();
            const predOpts = window.ChartModule.predictedSeries.options();
            return {
                realUpColor: realOpts.upColor,
                realDownColor: realOpts.downColor,
                predUpColor: predOpts.upColor,
                predDownColor: predOpts.downColor,
            };
        });

        expect(colors.realUpColor).toBe('#26a69a');
        expect(colors.realDownColor).toBe('#ef5350');
        expect(colors.predUpColor).toBe('rgba(38, 166, 154, 0.6)');
        expect(colors.predDownColor).toBe('rgba(239, 83, 80, 0.6)');
    });

    test('predicted series colors change by direction', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        const baseTime = Math.floor(Date.now() / 1000);
        await page.evaluate((candle) => {
            window.ChartModule.updateCandle(candle);
        }, {
            time: baseTime,
            open: 50000,
            high: 50100,
            low: 49900,
            close: 50050,
        });

        await page.evaluate(() => {
            window.ChartModule.updatePredictedCandle(
                { time: 1609459200, open: 50000, high: 51000, low: 49000, close: 50500 },
                'UP'
            );
        });

        let colors = await page.evaluate(() => {
            const opts = window.ChartModule.predictedSeries.options();
            return { upColor: opts.upColor, downColor: opts.downColor };
        });
        expect(colors.upColor).toBe('rgba(38, 166, 154, 0.6)');
        expect(colors.downColor).toBe('rgba(38, 166, 154, 0.6)');

        await page.evaluate(() => {
            window.ChartModule.updatePredictedCandle(
                { time: 1609459201, open: 50000, high: 51000, low: 49000, close: 49500 },
                'DOWN'
            );
        });

        colors = await page.evaluate(() => {
            const opts = window.ChartModule.predictedSeries.options();
            return { upColor: opts.upColor, downColor: opts.downColor };
        });
        expect(colors.upColor).toBe('rgba(239, 83, 80, 0.6)');
        expect(colors.downColor).toBe('rgba(239, 83, 80, 0.6)');

        await page.evaluate(() => {
            window.ChartModule.updatePredictedCandle(
                { time: 1609459202, open: 50000, high: 51000, low: 49000, close: 50000 },
                'UNCERTAIN'
            );
        });

        colors = await page.evaluate(() => {
            const opts = window.ChartModule.predictedSeries.options();
            return { upColor: opts.upColor, downColor: opts.downColor };
        });
        expect(colors.upColor).toBe('rgba(128, 128, 128, 0.3)');
        expect(colors.downColor).toBe('rgba(128, 128, 128, 0.3)');
    });
});

test.describe.serial('Prediction Message Handling', () => {
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

    async function waitForWSConnection(mockWSServer) {
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

    async function sendPrediction(mockWSServer, prediction) {
        mockWSServer.clients.forEach((client) => {
            if (client.readyState === WebSocket.OPEN) {
                client.send(JSON.stringify(prediction));
            }
        });
    }

    test('updates predicted series on prediction message', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        const baseTime = Math.floor(Date.now() / 1000);
        await page.evaluate((candle) => {
            window.ChartModule.updateCandle(candle);
        }, {
            time: baseTime,
            open: 50000,
            high: 50100,
            low: 49900,
            close: 50050,
        });

        await page.evaluate((wsUrl) => {
            if (window.WSClient) {
                window.WSClient.disconnect();
                window.WSClient.connect(wsUrl);
            }
        }, TEST_WS_URL);

        await waitForWSConnection(mockWSServer);

        const predictionTime = baseTime + 60;
        const testPrediction = {
            type: 'prediction',
            direction: 'UP',
            confidence: 0.85,
            predicted_candle: {
                open: 50050,
                high: 50200,
                low: 50000,
                close: 50150,
                close_time: predictionTime,
            },
            predicted_ma: 52000.5,
        };

        await sendPrediction(mockWSServer, testPrediction);
        await page.waitForTimeout(300);

        const predictedInfo = await page.evaluate(() => {
            const data = window.ChartModule.predictedCandles;
            return {
                dataLength: data ? data.length : 0,
                lastCandle: data && data.length > 0 ? data[data.length - 1] : null,
                hasWhitespace: data && data.some(function(c) {
                    return c && !c.hasOwnProperty('open') && !c.hasOwnProperty('close');
                }),
            };
        });

        expect(predictedInfo.dataLength).toBeGreaterThan(0);
        expect(predictedInfo.lastCandle).toMatchObject({
            time: predictionTime,
            open: 50050,
            high: 50200,
            low: 50000,
            close: 50150,
        });
        expect(predictedInfo.hasWhitespace).toBe(true);
    });

    test('test_up_prediction_is_green', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        const baseTime = Math.floor(Date.now() / 1000);
        await page.evaluate((candle) => {
            window.ChartModule.updateCandle(candle);
        }, {
            time: baseTime,
            open: 50000,
            high: 50100,
            low: 49900,
            close: 50050,
        });

        await page.evaluate((wsUrl) => {
            if (window.WSClient) {
                window.WSClient.disconnect();
                window.WSClient.connect(wsUrl);
            }
        }, TEST_WS_URL);

        await waitForWSConnection(mockWSServer);

        const predictionTime = baseTime + 60;
        await sendPrediction(mockWSServer, {
            type: 'prediction',
            direction: 'UP',
            confidence: 0.75,
            predicted_candle: {
                open: 50050,
                high: 50200,
                low: 50000,
                close: 50150,
                close_time: predictionTime,
            },
            predicted_ma: 52000.5,
        });

        await page.waitForTimeout(300);

        const colors = await page.evaluate(() => {
            const opts = window.ChartModule.predictedSeries.options();
            return {
                upColor: opts.upColor,
                downColor: opts.downColor,
                wickUpColor: opts.wickUpColor,
                wickDownColor: opts.wickDownColor,
            };
        });

        expect(colors.upColor).toBe('rgba(38, 166, 154, 0.6)');
        expect(colors.downColor).toBe('rgba(38, 166, 154, 0.6)');
        expect(colors.wickUpColor).toBe('rgba(38, 166, 154, 0.6)');
        expect(colors.wickDownColor).toBe('rgba(38, 166, 154, 0.6)');

        const confidenceText = await page.locator('#prediction-confidence').textContent();
        expect(confidenceText).toContain('UP');
        expect(confidenceText).toContain('75%');
    });

    test('test_down_prediction_is_red', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        const baseTime = Math.floor(Date.now() / 1000);
        await page.evaluate((candle) => {
            window.ChartModule.updateCandle(candle);
        }, {
            time: baseTime,
            open: 50000,
            high: 50100,
            low: 49900,
            close: 50050,
        });

        await page.evaluate((wsUrl) => {
            if (window.WSClient) {
                window.WSClient.disconnect();
                window.WSClient.connect(wsUrl);
            }
        }, TEST_WS_URL);

        await waitForWSConnection(mockWSServer);

        const predictionTime = baseTime + 60;
        await sendPrediction(mockWSServer, {
            type: 'prediction',
            direction: 'DOWN',
            confidence: 0.65,
            predicted_candle: {
                open: 50050,
                high: 50100,
                low: 49800,
                close: 49900,
                close_time: predictionTime,
            },
            predicted_ma: 49000.5,
        });

        await page.waitForTimeout(300);

        const colors = await page.evaluate(() => {
            const opts = window.ChartModule.predictedSeries.options();
            return {
                upColor: opts.upColor,
                downColor: opts.downColor,
                wickUpColor: opts.wickUpColor,
                wickDownColor: opts.wickDownColor,
            };
        });

        expect(colors.upColor).toBe('rgba(239, 83, 80, 0.6)');
        expect(colors.downColor).toBe('rgba(239, 83, 80, 0.6)');
        expect(colors.wickUpColor).toBe('rgba(239, 83, 80, 0.6)');
        expect(colors.wickDownColor).toBe('rgba(239, 83, 80, 0.6)');

        const confidenceText = await page.locator('#prediction-confidence').textContent();
        expect(confidenceText).toContain('DOWN');
        expect(confidenceText).toContain('65%');
    });

    test('test_uncertain_prediction_is_gray', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        const baseTime = Math.floor(Date.now() / 1000);
        await page.evaluate((candle) => {
            window.ChartModule.updateCandle(candle);
        }, {
            time: baseTime,
            open: 50000,
            high: 50100,
            low: 49900,
            close: 50050,
        });

        await page.evaluate((wsUrl) => {
            if (window.WSClient) {
                window.WSClient.disconnect();
                window.WSClient.connect(wsUrl);
            }
        }, TEST_WS_URL);

        await waitForWSConnection(mockWSServer);

        const predictionTime = baseTime + 60;
        await sendPrediction(mockWSServer, {
            type: 'prediction',
            direction: 'UNCERTAIN',
            confidence: 0.45,
            predicted_candle: {
                open: 50050,
                high: 50100,
                low: 50000,
                close: 50050,
                close_time: predictionTime,
            },
            predicted_ma: 50050.0,
        });

        await page.waitForTimeout(300);

        const colors = await page.evaluate(() => {
            const opts = window.ChartModule.predictedSeries.options();
            return {
                upColor: opts.upColor,
                downColor: opts.downColor,
                wickUpColor: opts.wickUpColor,
                wickDownColor: opts.wickDownColor,
            };
        });

        expect(colors.upColor).toBe('rgba(128, 128, 128, 0.3)');
        expect(colors.downColor).toBe('rgba(128, 128, 128, 0.3)');
        expect(colors.wickUpColor).toBe('rgba(128, 128, 128, 0.3)');
        expect(colors.wickDownColor).toBe('rgba(128, 128, 128, 0.3)');

        const confidenceText = await page.locator('#prediction-confidence').textContent();
        expect(confidenceText).toContain('UNCERTAIN');
        expect(confidenceText).toContain('45%');
    });

    test('test_predicted_candle_visible_after_70_seconds', async ({ page }) => {
        await page.goto('/');
        await page.waitForTimeout(500);

        const baseTime = Math.floor(Date.now() / 1000);
        for (let i = 0; i < 5; i++) {
            await page.evaluate((candle) => {
                window.ChartModule.updateCandle(candle);
            }, {
                time: baseTime + i,
                open: 50000 + i * 10,
                high: 50100 + i * 10,
                low: 49900 + i * 10,
                close: 50050 + i * 10,
            });
        }

        await page.evaluate((wsUrl) => {
            if (window.WSClient) {
                window.WSClient.disconnect();
                window.WSClient.connect(wsUrl);
            }
        }, TEST_WS_URL);

        await waitForWSConnection(mockWSServer);

        const predictionTime = baseTime + 70;
        const testPrediction = {
            type: 'prediction',
            direction: 'UP',
            confidence: 0.85,
            predicted_candle: {
                open: 50050,
                high: 50200,
                low: 50000,
                close: 50150,
                close_time: predictionTime,
            },
            predicted_ma: 52000.5,
        };

        await sendPrediction(mockWSServer, testPrediction);
        await page.waitForTimeout(300);

        const predictedInfo = await page.evaluate(() => {
            const data = window.ChartModule.predictedCandles;
            return {
                dataLength: data ? data.length : 0,
                lastCandle: data && data.length > 0 ? data[data.length - 1] : null,
                hasWhitespace: data && data.some(function(c) {
                    return c && !c.hasOwnProperty('open') && !c.hasOwnProperty('close');
                }),
            };
        });

        expect(predictedInfo.dataLength).toBeGreaterThan(0);
        expect(predictedInfo.lastCandle).toMatchObject({
            time: predictionTime,
            open: 50050,
            high: 50200,
            low: 50000,
            close: 50150,
        });
        expect(predictedInfo.hasWhitespace).toBe(true);
    });

});
