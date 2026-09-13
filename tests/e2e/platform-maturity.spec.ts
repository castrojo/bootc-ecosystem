import { test, expect } from '@playwright/test';
import type { Page } from '@playwright/test';

async function getScriptJSON(page: Page, id: string): Promise<unknown> {
  const result = await page.evaluate((scriptId: string) => {
    const el = document.getElementById(scriptId);
    if (!el) return { error: `Element #${scriptId} not found in DOM` };
    const content = el.textContent ?? '';
    try {
      return { data: JSON.parse(content) };
    } catch (e) {
      return { error: `JSON.parse failed: ${String(e)}`, preview: content.slice(0, 200) };
    }
  }, id);

  const r = result as { error?: string; preview?: string; data?: unknown };
  if (r.error) {
    throw new Error(`script#${id}: ${r.error}${r.preview ? `\nContent: ${r.preview}` : ''}`);
  }
  return r.data;
}

test.describe('Platform Maturity page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/bootc-ecosystem/platform-maturity/');
    await page.waitForLoadState('networkidle');
  });

  test('page loads with correct title', async ({ page }) => {
    await expect(page).toHaveTitle(/Platform Maturity/);
  });

  test('h1 renders self-assessment heading', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('Platform Engineering Maturity Self-Assessment');
  });

  test('platform-maturity tab is active in navigation', async ({ page }) => {
    const tabLink = page.locator('nav a[href*="platform-maturity"]');
    await expect(tabLink).toBeVisible();
  });

  test('platform-maturity-chart-data has valid JSON with a non-empty series', async ({ page }) => {
    const data = await getScriptJSON(page, 'platform-maturity-chart-data') as Record<string, unknown>;
    expect(Array.isArray(data.series), 'series must be an array').toBe(true);
    expect((data.series as unknown[]).length, 'series must be non-empty').toBeGreaterThan(0);
  });

  test('radar chart canvas is rendered by Chart.js', async ({ page }) => {
    const canvas = page.locator('canvas#platform-maturity-chart');
    await expect(canvas, 'canvas#platform-maturity-chart must exist').toBeAttached();
    const box = await canvas.boundingBox();
    expect(box, 'canvas must have a bounding box — Chart.js did not render').not.toBeNull();
    expect(box!.width).toBeGreaterThan(0);
    expect(box!.height).toBeGreaterThan(0);
  });

  test('dimension table is visible with expected columns and rows', async ({ page }) => {
    const table = page.locator('#platform-maturity-table');
    await expect(table).toBeVisible();
    const headers = table.locator('thead th');
    await expect(headers).toHaveText(['Dimension', 'Level', 'Evidence', 'Last Assessed']);
    const rows = table.locator('tbody tr');
    await expect(rows.first()).toBeVisible();
    expect(await rows.count()).toBeGreaterThanOrEqual(7);
  });

  test('all JSON data scripts are parseable', async ({ page }) => {
    const scripts = await page.evaluate(() => {
      const els = Array.from(document.querySelectorAll('script[type="application/json"]'));
      return els.map(el => ({
        id: el.id,
        valid: (() => { try { JSON.parse(el.textContent ?? ''); return true; } catch { return false; } })()
      }));
    });
    for (const s of scripts) {
      expect(s.valid).toBe(true);
    }
  });
});
