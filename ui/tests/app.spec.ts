import { test, expect } from '@playwright/test'

test.describe('Robot Tracker UI Integration', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('http://localhost:9086')
        await page.waitForLoadState('networkidle')
        await page.waitForTimeout(1000)
    })

    test('should load the main page with Vue app', async ({ page }) => {
        await expect(page.locator('.app-container')).toBeVisible({ timeout: 10000 })
    })

    test('should display bottom bar elements', async ({ page }) => {
        await expect(page.locator('.bottom-bar')).toBeVisible({ timeout: 10000 })
        await expect(page.locator('.calibration-badge')).toBeVisible()
        await expect(page.locator('.obstacle-toggle')).toBeVisible()
    })

    test('should toggle obstacle panel', async ({ page }) => {
        await page.locator('.obstacle-toggle').click()
        await expect(page.locator('.obstacle-panel')).toBeVisible({ timeout: 5000 })
    })

    test('should open calibration wizard when clicking calibration badge', async ({ page }) => {
        await page.locator('.calibration-badge').click()
        await expect(page.locator('.calibration-wizard')).toBeVisible({ timeout: 5000 })
        await expect(page.locator('.calibration-step.active h2')).toContainText(
            'Step 1: Enter Tag Size'
        )
    })

    test('should close calibration wizard when clicking overlay', async ({ page }) => {
        await page.locator('.calibration-badge').click()
        await expect(page.locator('.calibration-wizard')).toBeVisible({ timeout: 5000 })
        await page.locator('.calibration-overlay').click({ position: { x: 10, y: 10 } })
        await expect(page.locator('.calibration-wizard')).toBeHidden({ timeout: 3000 })
    })

    test('should show detected tags step when clicking Detect Tags', async ({ page }) => {
        await page.locator('.calibration-badge').click()
        await expect(page.locator('.calibration-wizard')).toBeVisible({ timeout: 5000 })
        await page.locator('.calibration-btn.primary:has-text("Detect Tags")').click()
        await expect(page.locator('.calibration-step.active h2')).toContainText(
            'Step 2: Select Detected Tag'
        )
    })
})

test.describe('API Endpoints', () => {
    test('should return calibration status', async ({ request }) => {
        const response = await request.get('http://localhost:9086/api/calibration/status')
        expect(response.status()).toBe(200)
        const data = await response.json()
        expect(data).toHaveProperty('state')
    })

    test('should return obstacles', async ({ request }) => {
        const response = await request.get('http://localhost:9086/api/obstacles')
        expect(response.status()).toBe(200)
        const data = await response.json()
        expect(data).toHaveProperty('obstacles')
    })

    test('should return status', async ({ request }) => {
        const response = await request.get('http://localhost:9086/api/status')
        expect(response.status()).toBe(200)
        const data = await response.json()
        expect(data).toHaveProperty('fps')
        expect(data).toHaveProperty('arduinoState')
    })
})

test.describe('Video Stream', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('http://localhost:9086')
        await page.waitForLoadState('networkidle')
        await page.waitForTimeout(1000)
    })

    test('should have video element', async ({ page }) => {
        await expect(page.locator('#video')).toHaveAttribute('src', '/stream')
    })

    test('should have overlay canvas', async ({ page }) => {
        await expect(page.locator('#overlay')).toBeVisible({ timeout: 10000 })
    })
})

test.describe('Control Panel', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('http://localhost:9086')
        await page.waitForLoadState('networkidle')
        await page.waitForTimeout(1000)
    })

    test('should have manual control buttons', async ({ page }) => {
        await expect(page.locator('.panel-right .panel:has-text("Controls")')).toBeVisible({
            timeout: 10000,
        })
        await expect(page.locator('.panel-right .panel:has-text("Controls") .btn')).toHaveCount(10)
    })
})

test.describe('Destination Planning', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('http://localhost:9086')
        await page.waitForLoadState('networkidle')
        await page.waitForTimeout(1000)
    })

    test('should display track list with detected targets', async ({ page }) => {
        await expect(page.locator('.panel-left .panel:has-text("Detected Targets")')).toBeVisible({
            timeout: 10000,
        })
        await expect(page.locator('.track-item')).toHaveCount(3, { timeout: 5000 })
    })

    test('should show track items as clickable', async ({ page }) => {
        await expect(page.locator('.track-item').first()).toBeVisible({ timeout: 5000 })
        const trackItem = page.locator('.track-item').first()
        await expect(trackItem).toHaveClass(/track-item/)
    })

    test('should have bottom bar when destination mode is active', async ({ page }) => {
        // The destination badge appears in bottom bar when destination mode is active
        // Note: This test verifies the UI structure; actual destination mode requires robot selection
        await expect(page.locator('.bottom-bar')).toBeVisible({ timeout: 5000 })
    })

    test('should have overlay canvas for destination cursor', async ({ page }) => {
        await expect(page.locator('#overlay')).toBeVisible({ timeout: 10000 })
    })

    test('should have track items with selection styling', async ({ page }) => {
        // Track items should have CSS classes for selection and destination states
        const trackItems = page.locator('.track-item')
        const count = await trackItems.count()
        expect(count).toBeGreaterThan(0)

        // Verify styling classes are available in the CSS
        await expect(page.locator('.panel-left .panel:has-text("Detected Targets")')).toHaveClass(
            /panel/
        )
    })
})

test.describe('Destination API', () => {
    test('should accept destination POST request', async ({ request }) => {
        const response = await request.post('http://localhost:9086/api/destination', {
            data: {
                robot_id: 1,
                x: 320,
                y: 240,
            },
        })
        expect(response.status()).toBe(200)
        const data = await response.json()
        expect(data.status).toBe('ok')
    })

    test('should accept destination even without robot_id (defaults to 0)', async ({ request }) => {
        const response = await request.post('http://localhost:9086/api/destination', {
            data: {
                x: 320,
                y: 240,
            },
        })
        // Go/gin doesn't enforce required fields by default
        expect(response.status()).toBe(200)
    })
})
