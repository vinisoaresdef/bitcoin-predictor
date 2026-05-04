const { test, expect } = require('@playwright/test');

test.describe('Predictor Platform Frontend', () => {
    test('page loads with chart container and dark background', async ({ page }) => {
        await page.goto('/');

        const chartContainer = page.locator('#chart-container');
        await expect(chartContainer).toBeVisible();

        const containerBox = await chartContainer.boundingBox();
        expect(containerBox.height).toBeGreaterThan(400);

        const body = page.locator('body');
        const bgColor = await body.evaluate(el => {
            return window.getComputedStyle(el).backgroundColor;
        });
        expect(bgColor).toBe('rgb(26, 26, 46)');

        const pageErrors = [];
        page.on('pageerror', err => pageErrors.push(err.message));
        await page.waitForTimeout(500);
        expect(pageErrors).toEqual([]);
    });

    test('status bar is visible at bottom with connecting state', async ({ page }) => {
        await page.goto('/');

        const statusBar = page.locator('#status-bar');
        await expect(statusBar).toBeVisible();

        const statusText = page.locator('#status-text');
        await expect(statusText).toContainText('Connecting');

        const statusBarBox = await statusBar.boundingBox();
        const viewportHeight = page.viewportSize().height;
        expect(statusBarBox.y + statusBarBox.height).toBeGreaterThanOrEqual(viewportHeight - 5);

        const hasConnectingClass = await statusBar.evaluate(el =>
            el.classList.contains('status-connecting')
        );
        expect(hasConnectingClass).toBe(true);
    });
});
