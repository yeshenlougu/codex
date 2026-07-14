#!/usr/bin/env python3
"""Take screenshots by clicking sidebar nav items in Codex Go."""
import asyncio
from pathlib import Path

from playwright.async_api import async_playwright

OUTPUT_DIR = Path("/home/ubuntu/app/codex/screenshots")
BASE_URL = "http://localhost:5173"

# Nav items in order, with click strategy
# We must be on 'chat' page for sidebar to be visible
PAGES = [
    # (page_name, click_strategy, description)
    ("chat", "first_nav", "Chat home page"),
    ("scheduled", "second_nav", "Scheduled tasks page"),
    ("plugins", "third_nav", "Plugins/MCP/Skills page"),
    ("settings", "settings_gear", "Settings page"),
]

async def click_nav_item(page, index: int):
    """Click the nth nav item in the sidebar. 0=chat, 1=scheduled, 2=plugins."""
    selector = f"div[style*='display: flex'][style*='align-items: center'][style*='gap: 8px']:nth-of-type({index + 2})"
    # Actually, let's try a simpler approach: find all clickable nav divs
    nav_items = await page.locator("text=新建任务, text=已安排, text=插件").all()
    if index < len(nav_items):
        await nav_items[index].click()
        return True
    return False

async def main():
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        context = await browser.new_context(
            viewport={"width": 1440, "height": 900},
            device_scale_factor=2,
        )
        page = await context.new_page()

        # Load app
        print("Loading app...")
        await page.goto(BASE_URL, wait_until="domcontentloaded", timeout=20000)
        try:
            await page.wait_for_function(
                "document.getElementById('root')?.children?.length > 0",
                timeout=15000
            )
            print("React mounted OK\n")
        except Exception as e:
            print(f"React mount failed: {e}")
            return

        await page.wait_for_timeout(2000)

        # Screenshot 1: Chat home page (current state)
        print("📸 Chat page (home)")
        await page.screenshot(path=str(OUTPUT_DIR / "chat.png"), full_page=False)
        print(f"  Saved: chat.png ({OUTPUT_DIR.joinpath('chat.png').stat().st_size:,} bytes)\n")

        # Get all nav items
        nav_labels = ["新建任务", "已安排", "插件"]
        nav_locators = []

        for label in nav_labels:
            try:
                # Use a broader locator - look for divs containing this text
                loc = page.locator(f"text={label}").first
                nav_locators.append(loc)
            except:
                nav_locators.append(None)

        for i, (label, loc) in enumerate(zip(nav_labels, nav_locators)):
            page_name = ["chat", "scheduled", "plugins"][i]
            print(f"📸 {page_name} page — clicking '{label}'")

            # First navigate back to chat so sidebar is visible
            if i > 0:
                # Click "新建任务" to go back to chat
                try:
                    await nav_locators[0].click(timeout=3000)
                    await page.wait_for_timeout(1000)
                except:
                    # Try reloading
                    await page.goto(BASE_URL, wait_until="domcontentloaded", timeout=10000)
                    await page.wait_for_timeout(2000)

            # Now click the target nav item
            try:
                await loc.click(timeout=5000)
                await page.wait_for_timeout(1500)
            except Exception as e:
                print(f"  Click failed: {e}, trying reload approach...")
                # Reload and try again
                await page.goto(BASE_URL, wait_until="domcontentloaded", timeout=10000)
                await page.wait_for_timeout(2000)
                try:
                    await loc.click(timeout=5000)
                    await page.wait_for_timeout(1500)
                except Exception as e2:
                    print(f"  Still failed: {e2}")

            path = OUTPUT_DIR / f"{page_name}.png"
            await page.screenshot(path=str(path), full_page=False)
            print(f"  Saved: {page_name}.png ({path.stat().st_size:,} bytes)\n")

        # Settings page — click gear icon
        print("📸 settings page — clicking gear icon")
        # Go back to chat first
        try:
            await nav_locators[0].click(timeout=3000)
            await page.wait_for_timeout(1000)
        except:
            await page.goto(BASE_URL, wait_until="domcontentloaded", timeout=10000)
            await page.wait_for_timeout(2000)

        # Try multiple selectors for the settings gear
        settings_clicked = False
        for sel in [".anticon-setting", "svg[data-icon='setting']", "[aria-label='setting']"]:
            try:
                await page.click(sel, timeout=3000)
                await page.wait_for_timeout(1500)
                settings_clicked = True
                break
            except:
                continue

        if not settings_clicked:
            # Try clicking the bottom area of the sidebar
            try:
                sidebar = page.locator(".ant-layout-sider")
                box = await sidebar.bounding_box()
                if box:
                    # Click bottom center of sidebar
                    await page.mouse.click(box["x"] + box["width"] / 2, box["y"] + box["height"] - 30)
                    await page.wait_for_timeout(1500)
            except:
                pass

        path = OUTPUT_DIR / "settings.png"
        await page.screenshot(path=str(path), full_page=False)
        print(f"  Saved: settings.png ({path.stat().st_size:,} bytes)\n")

        # Also capture full-page versions
        for name in ["chat", "scheduled", "plugins", "settings"]:
            # Navigate back first, then to the page
            pass  # Already captured

        await browser.close()
        print("✅ All screenshots saved to", OUTPUT_DIR)

if __name__ == "__main__":
    asyncio.run(main())
