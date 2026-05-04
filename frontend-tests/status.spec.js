const { test, expect } = require('@playwright/test');

test.describe('Status Bar Transitions', () => {
    test('test_status_transitions', async ({ page }) => {
        await page.goto('/');

        const statusBar = page.locator('#status-bar');
        const statusText = page.locator('#status-text');
        const loaderOverlay = page.locator('#loader-overlay');

        await expect(statusBar).toBeVisible();
        await expect(statusText).toBeVisible();
        await expect(loaderOverlay).toBeVisible();

        // Helper to simulate a WS status message
        const simulateStatus = async (status, bufferSize) => {
            await page.evaluate(({ status, bufferSize }) => {
                if (window.StatusHandler && window.StatusHandler.updateStatus) {
                    window.StatusHandler.updateStatus({
                        type: 'status',
                        status: status,
                        buffer_size: bufferSize
                    });
                }
            }, { status, bufferSize });
        };

        // 1. connected → green, "Live", hide loader
        await simulateStatus('connected', 60);
        await expect(statusBar).toHaveClass(/status-connected/);
        await expect(statusText).toHaveText('Live');
        await expect(loaderOverlay).toHaveClass(/hidden/);

        // 2. collecting → yellow, "Collecting data (X/60)", show loader
        await simulateStatus('collecting', 25);
        await expect(statusBar).toHaveClass(/status-collecting/);
        await expect(statusText).toContainText('Collecting data (25/60)');
        const loaderHiddenCollecting = await loaderOverlay.evaluate(el =>
            el.classList.contains('hidden')
        );
        expect(loaderHiddenCollecting).toBe(false);

        // Verify loader text shows buffer progress
        const loaderTextCollecting = page.locator('.loader-text');
        await expect(loaderTextCollecting).toContainText('25/60 candles');

        // 3. reconnecting → yellow, "Reconnecting...", show loader
        await simulateStatus('reconnecting', 0);
        await expect(statusBar).toHaveClass(/status-reconnecting/);
        await expect(statusText).toHaveText('Reconnecting...');
        const loaderHiddenReconnecting = await loaderOverlay.evaluate(el =>
            el.classList.contains('hidden')
        );
        expect(loaderHiddenReconnecting).toBe(false);
        await expect(loaderTextCollecting).toHaveText('Reconnecting...');

        // 4. disconnected → red, "Disconnected", show loader
        await simulateStatus('disconnected', 0);
        await expect(statusBar).toHaveClass(/status-disconnected/);
        await expect(statusText).toHaveText('Disconnected');
        const loaderHiddenDisconnected = await loaderOverlay.evaluate(el =>
            el.classList.contains('hidden')
        );
        expect(loaderHiddenDisconnected).toBe(false);
    });
});
