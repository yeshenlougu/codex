import { test, expect } from '@playwright/test';

test.describe('Theme toggle', () => {
  test('defaults to system preference and sets data-theme', async ({ page }) => {
    await page.goto('/');

    // Wait for the app to mount
    await page.waitForSelector('.app-root');

    // data-theme attribute must be set
    const theme = await page.evaluate(() =>
      document.documentElement.getAttribute('data-theme')
    );
    expect(['dark', 'light']).toContain(theme);
  });

  test('toggle button switches theme and persists', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.app-root');

    // Read current theme
    const before = await page.evaluate(() =>
      document.documentElement.getAttribute('data-theme')
    );

    // Click the theme toggle button in the titlebar
    const toggleBtn = page.locator('.titlebar-btn').filter({ hasText: /[☀☾]/ });
    await toggleBtn.click();

    // Theme should have changed
    const after = await page.evaluate(() =>
      document.documentElement.getAttribute('data-theme')
    );
    expect(after).not.toBe(before);

    // Reload — theme should persist via localStorage
    await page.reload();
    await page.waitForSelector('.app-root');

    const persisted = await page.evaluate(() =>
      document.documentElement.getAttribute('data-theme')
    );
    expect(persisted).toBe(after);
  });

  test('toggle icon flips with theme', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.app-root');

    const toggleBtn = page.locator('.titlebar-btn').filter({ hasText: /[☀☾]/ });
    const iconBefore = await toggleBtn.textContent();

    await toggleBtn.click();
    const iconAfter = await toggleBtn.textContent();

    expect(iconAfter).not.toBe(iconBefore);
  });
});
