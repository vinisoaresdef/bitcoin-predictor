const { test, expect } = require('@playwright/test');

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
