import { test, expect } from '@playwright/test'

test('should load the main page with Vue app', async ({ page }) => {
  await page.goto('http://localhost:5173')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000)
  await expect(page.locator('.app-container')).toBeVisible({ timeout: 15000 })
  await expect(page.locator('h1')).toContainText('Robot Tracker')
})

test('should display header elements', async ({ page }) => {
  await page.goto('http://localhost:5173')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000)
  await expect(page.locator('.calibration-badge')).toBeVisible({ timeout: 15000 })
  await expect(page.locator('.obstacle-toggle')).toBeVisible()
})

test('should show connecting state when not connected', async ({ page }) => {
  await page.goto('http://localhost:5173')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000)
  await expect(page.locator('.loading')).toBeVisible({ timeout: 15000 })
})
