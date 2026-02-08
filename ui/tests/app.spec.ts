import { test, expect } from '@playwright/test'

test.describe('Robot Tracker UI Integration', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('http://localhost:9086')
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)
  })

  test('should load the main page with Vue app', async ({ page }) => {
    await expect(page.locator('.app-container')).toBeVisible({ timeout: 10000 })
    await expect(page.locator('h1')).toContainText('Robot Tracker')
  })

  test('should display header elements', async ({ page }) => {
    await expect(page.locator('.calibration-badge')).toBeVisible({ timeout: 10000 })
    await expect(page.locator('.obstacle-toggle')).toBeVisible()
  })

  test('should toggle obstacle panel', async ({ page }) => {
    await page.locator('.obstacle-toggle').click()
    await expect(page.locator('.obstacle-panel')).toBeVisible({ timeout: 5000 })
  })

  test('should open calibration wizard when clicking Not Calibrated badge', async ({ page }) => {
    await page.locator('.calibration-badge.not-calibrated').click()
    await expect(page.locator('.calibration-wizard')).toBeVisible({ timeout: 5000 })
    await expect(page.locator('.calibration-step.active h2')).toContainText('Step 1: Enter Tag Size')
  })

  test('should close calibration wizard when clicking overlay', async ({ page }) => {
    await page.locator('.calibration-badge.not-calibrated').click()
    await expect(page.locator('.calibration-wizard')).toBeVisible({ timeout: 5000 })
    await page.locator('.calibration-overlay').click({ position: { x: 10, y: 10 } })
    await expect(page.locator('.calibration-wizard')).toBeHidden({ timeout: 3000 })
  })

  test('should show detected tags step when clicking Detect Tags', async ({ page }) => {
    await page.locator('.calibration-badge.not-calibrated').click()
    await expect(page.locator('.calibration-wizard')).toBeVisible({ timeout: 5000 })
    await page.locator('.calibration-btn.primary:has-text("Detect Tags")').click()
    await expect(page.locator('.calibration-step.active h2')).toContainText('Step 2: Select Detected Tag')
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
    await expect(page.locator('.panel:has-text("Controls")')).toBeVisible({ timeout: 10000 })
    await expect(page.locator('.panel:has-text("Controls") .btn')).toHaveCount(5)
  })
})
