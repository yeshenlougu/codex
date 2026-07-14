#!/usr/bin/env python3
"""Debug why React isn't rendering in headless browser."""
import asyncio
from playwright.async_api import async_playwright

BASE_URL = "http://localhost:5173"

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        context = await browser.new_context(
            viewport={"width": 1440, "height": 900},
        )
        page = await context.new_page()

        # Capture ALL console messages
        console = []
        page.on("console", lambda msg: console.append(f"[{msg.type}] {msg.text}"))
        page.on("pageerror", lambda err: console.append(f"[PAGE_ERROR] {err}"))

        print("Loading page...")
        resp = await page.goto(BASE_URL, wait_until="domcontentloaded", timeout=30000)
        print(f"Response status: {resp.status}")
        print(f"Response headers: {dict(resp.headers)}")

        # Wait and collect console
        await asyncio.sleep(5)

        print("\n=== Console messages ===")
        for m in console:
            print(m)

        print("\n=== DOM body ===")
        body = await page.evaluate("document.body.innerHTML")
        print(body[:1000])

        # Check root div
        root_html = await page.evaluate("document.getElementById('root')?.innerHTML || 'EMPTY'")
        print(f"\n=== Root div: {root_html[:500]} ===")

        await browser.close()

asyncio.run(main())
