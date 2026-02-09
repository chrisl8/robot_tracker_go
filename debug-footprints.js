const { chromium } = require('playwright');

(async () => {
    const browser = await chromium.launch();
    const page = await browser.newPage();

    // Collect console messages
    const consoleMessages = [];
    page.on('console', msg => {
        consoleMessages.push({ type: msg.type(), text: msg.text() });
    });

    try {
        // Navigate to the page
        await page.goto('http://localhost:9086', { waitUntil: 'networkidle', timeout: 30000 });

        // Wait for video and overlay
        await page.waitForSelector('#video', { timeout: 10000 });
        await page.waitForSelector('#overlay', { timeout: 10000 });

        // Wait a bit for tracks to be processed
        await page.waitForTimeout(3000);

        // Get track info from the store
        const trackInfo = await page.evaluate(() => {
            // @ts-ignore
            const robotStore = window.__piniaStores?.robot;
            if (robotStore) {
                return {
                    tracks: JSON.parse(JSON.stringify(robotStore.tracks || [])),
                    confirmedTracks: JSON.parse(JSON.stringify(robotStore.confirmedTracks || [])),
                    trackCount: robotStore.trackCount || 0,
                    confirmedCount: robotStore.confirmedCount || 0
                };
            }
            return { error: 'Store not found' };
        });

        console.log('=== Track Info ===');
        console.log(JSON.stringify(trackInfo, null, 2));

        // Check showFootprints toggle
        const showFootprints = await page.evaluate(() => {
            // @ts-ignore
            const uiStore = window.__piniaStores?.ui;
            return uiStore?.showFootprints;
        });
        console.log('\n=== UI Store ===');
        console.log('showFootprints:', showFootprints);

        // Take screenshot
        await page.screenshot({ path: '/tmp/robot-tracker.png', fullPage: true });
        console.log('\nScreenshot saved to /tmp/robot-tracker.png');

        // Print console messages related to footprints
        console.log('\n=== Console Messages ===');
        const footprintMsgs = consoleMessages.filter(m =>
            m.text.includes('Footprint') ||
            m.text.includes('track') ||
            m.text.includes('Track')
        );
        footprintMsgs.forEach(m => console.log(`[${m.type}] ${m.text}`));

    } catch (error) {
        console.error('Error:', error.message);
    } finally {
        await browser.close();
    }
})();
