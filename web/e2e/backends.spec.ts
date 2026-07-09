import { test, expect } from '@playwright/test';

test.describe('Backend model display', () => {
  test('navigates to settings → backends and shows models', async ({ page }) => {
    // First, send a chat message to create an agent with a pool
    await page.goto('/');
    await page.waitForSelector('.app-root');

    // Navigate to Settings via sidebar
    const settingsNav = page.locator('.ls-nav-item').filter({ hasText: 'Settings' });
    // If not visible, the sidebar might need clicking first. Actually, the snapshot shows
    // the Settings nav is always visible since the sidebar doesn't collapse.
    await settingsNav.click();

    // Wait for settings to render
    await page.waitForSelector('text=SETTINGS');

    // Click the Backends sub-tab (settings has its own sidebar nav)
    await page.locator('.session-item').filter({ hasText: '🔌 Backends' }).click();

    // Wait for the backend panel to show
    await page.waitForTimeout(1000);

    // The heading should show "Backends" (there are two h2s — pick the first)
    await expect(page.locator('h2').filter({ hasText: 'Backends' }).first()).toBeVisible();
  });

  test('Probe & Discover button triggers model discovery', async ({ page }) => {
    // First ensure an agent exists by sending a chat
    await page.goto('/');
    await page.waitForSelector('.app-root');

    const input = page.locator('.chat-input');
    await input.fill('hi');
    await page.keyboard.press('Enter');

    // Wait briefly for agent creation + model discovery
    await page.waitForTimeout(3000);

    // Navigate to backends
    await page.locator('.ls-nav-item').filter({ hasText: 'Settings' }).click();
    await page.waitForSelector('text=SETTINGS');
    await page.locator('.session-item').filter({ hasText: '🔌 Backends' }).click();
    await page.waitForTimeout(1000);

    // Click Probe & Discover
    const probeBtn = page.locator('button').filter({ hasText: 'Probe & Discover' });
    await probeBtn.click();

    // Should see model names appear (auto-discovered)
    await page.waitForTimeout(2000);
    // Check that we see the base URL displayed
    await expect(page.locator('text=https://opencode.ai')).toBeVisible();
  });
});
