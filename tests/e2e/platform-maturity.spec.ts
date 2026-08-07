import { test, expect, type Page } from '@playwright/test';

/**
 * Platform Maturity page — rendering tests.
 *
 * The page renders from the hand-maintained static data file
 * src/data/platform-maturity.json, so these are RENDERING tests
 * (structure + chart), safe to run pre-deploy on any data state.
 */

/** Extract and parse JSON from a SSR'd <script type="application/json"> element. */
async function getScriptJSON(page: Page, id: string): Promise<unknown> {
  const result = await page.evaluate((scriptId: string) => {
    const el = document.getElementById(scriptId);
    if (!el) return { error: `Element #${scriptId} not found in DOM` };
    try {
      return { data: JSON.parse(el.textContent ?? '') };
    } catch (e) {
      return { error: `JSON.parse failed: ${String(e)}` };
    }
  }, id);
  const r = result as { error?: string; data?: unknown };
  if (r.error) throw new Error(`script#${id}: ${r.error}`);
  return r.data;
}

test.describe('Platform Maturity page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/bootc-ecosystem/platform-maturity/');
    await page.waitForLoadState('networkidle');
  });

  test('radar chart data script has valid JSON with all seven dimensions', async ({ page }) => {
    const data = await getScriptJSON(page, 'platform-maturity-radar-data') as {
      dimensions?: Array<{ key: string; label: string; level: number }>;
    };
    expect(Array.isArray(data.dimensions), 'dimensions must be an array').toBe(true);
    expect(data.dimensions!.length, 'must have all 7 CNCF dimensions').toBe(7);
  });

  test('radar chart canvas is rendered by Chart.js', async ({ page }) => {
    const canvas = page.locator('canvas#platform-maturity-radar-chart');
    await expect(canvas, 'radar canvas must exist').toBeAttached();
    const box = await canvas.boundingBox();
    expect(box, 'radar canvas must have a bounding box — Chart.js did not render').not.toBeNull();
    expect(box!.width, 'radar canvas width must be > 0').toBeGreaterThan(0);
    expect(box!.height, 'radar canvas height must be > 0').toBeGreaterThan(0);
  });

  test('dimension table renders a row per dimension with levels and dates', async ({ page }) => {
    const rows = page.locator('.maturity-table tbody tr');
    expect(await rows.count(), 'dimension table must have 7 rows').toBe(7);
    const firstRowCells = rows.first().locator('td');
    expect(await firstRowCells.count(), 'rows must have 5 cells (dimension, level, evidence, gap, date)').toBe(5);
    // Last cell is the last-assessed date in ISO format
    const dateText = await firstRowCells.nth(4).textContent();
    expect(dateText?.trim(), 'last-assessed date must be YYYY-MM-DD').toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  test('page mentions the CNCF model and the last assessed date', async ({ page }) => {
    await expect(page.locator('.meta')).toContainText('CNCF Platform Engineering Maturity Model');
    await expect(page.locator('.meta')).toContainText(/Last assessed \d{4}-\d{2}-\d{2}/);
  });

  test('TabNav contains the Platform Maturity tab', async ({ page }) => {
    const tab = page.locator('.tab-nav a.tab', { hasText: 'Platform Maturity' });
    await expect(tab).toBeVisible();
    await expect(tab).toHaveClass(/active/);
  });

  test('page has no empty charts', async ({ page }) => {
    const emptyCharts = await page.locator('.chart-empty').all();
    expect(emptyCharts.length, 'platform-maturity page must not render empty charts').toBe(0);
  });
});
