import { test, expect } from '@playwright/test'

test('debug page load', async ({ page }) => {
  const failedRequests: string[] = []
  
  page.on('requestfailed', request => {
    failedRequests.push(`Failed: ${request.url()} - ${request.failure()?.errorText}`)
  })
  
  page.on('response', response => {
    if (response.status() >= 400) {
      failedRequests.push(`HTTP ${response.status()}: ${response.url()}`)
    }
  })
  
  await page.goto('http://localhost:9086', { waitUntil: 'networkidle' })
  await page.waitForTimeout(2000)
  
  console.log('=== Failed Requests ===')
  failedRequests.forEach(r => console.log(r))
  
  console.log('=== #app content ===')
  console.log(await page.locator('#app').innerHTML())
})
