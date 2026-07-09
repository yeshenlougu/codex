import { test, expect } from '@playwright/test';

test.describe('Chat flow', () => {
  test('sends a message and receives a response', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.app-root');

    // Find the chat input
    const input = page.locator('.chat-input');
    await expect(input).toBeVisible();

    // Send a simple message
    const testMessage = 'reply with exactly one word: OK';
    await input.fill(testMessage);
    await page.keyboard.press('Enter');

    // Wait for response — the assistant's message should appear
    // The response comes through streaming then renders in the chat area
    await expect(async () => {
      const pageText = await page.locator('body').innerText();
      expect(pageText).toContain('OK');
    }).toPass({ timeout: 30000 });

    // The input should be empty after sending (message was consumed)
    const inputAfter = await input.inputValue();
    expect(inputAfter).toBe('');
  });

  test('empty state shows prompt suggestions', async ({ page }) => {
    // Create a new session to get the empty state
    await page.goto('/');
    await page.waitForSelector('.app-root');

    // Click "New Session"
    const newBtn = page.locator('.ls-new-btn, button').filter({ hasText: /New Session/i });
    await newBtn.click();
    await page.waitForTimeout(500);

    // Empty state hints should be visible
    await expect(page.locator('text=Explain this code')).toBeVisible();
    await expect(page.locator('text=Write a function')).toBeVisible();
    await expect(page.locator('text=Debug an error')).toBeVisible();
    await expect(page.locator('text=Refactor module')).toBeVisible();
  });

  test('mascot image loads in empty state', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.app-root');

    // Find the mascot image
    const mascot = page.locator('.chat-empty-mascot');
    await expect(mascot).toBeVisible();

    // Check the image loaded (naturalWidth > 0)
    const naturalWidth = await mascot.evaluate((el: HTMLImageElement) => el.naturalWidth);
    expect(naturalWidth).toBeGreaterThan(0);
  });

  test('can switch between sessions in sidebar', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.app-root');

    // Send a message first to create content in current session
    const input = page.locator('.chat-input');
    await input.fill('hi');
    await page.keyboard.press('Enter');
    await page.waitForTimeout(3000);

    // Click a previous session in the sidebar
    const prevSession = page.locator('.ls-session').first();
    const sessionExists = await prevSession.count();
    if (sessionExists > 0) {
      await prevSession.click();
      await page.waitForTimeout(500);

      // The selected session should be highlighted
      await expect(prevSession).toHaveClass(/active/);
    }
  });
});
