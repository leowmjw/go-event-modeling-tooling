import { chromium } from 'playwright';

const url = 'http://localhost:8080/';

const browser = await chromium.launch();
const page = await browser.newPage();

await page.goto(url, { waitUntil: 'networkidle' });
await page.waitForFunction(() => document.querySelector('form.picker select'));

const modelSelect = page.locator('header select').first();
const defaultModel = await modelSelect.inputValue();
const options = await modelSelect.locator('option').all();
let chosenModel = defaultModel;
for (const opt of options) {
  const v = await opt.getAttribute('value');
  if (v && v !== defaultModel) {
    chosenModel = v;
    await modelSelect.selectOption(v);
    break;
  }
}
await page.waitForTimeout(500);

const flowSelect = page.locator('form.picker select');
const flowOptions = await flowSelect.locator('option').all();
let targetFlow = null;
for (const opt of flowOptions) {
  const v = await opt.getAttribute('value');
  if (v && v !== '' && v !== '__new__') {
    targetFlow = v;
    break;
  }
}
if (!targetFlow) throw new Error('no fixture flow found');

await flowSelect.selectOption(targetFlow);
await page.locator('form.picker button[type=submit]').click();
await page.waitForSelector('.draft-tabs', { timeout: 10000 });
await page.waitForTimeout(1000);

const svgCount = await page.locator('#svg-container svg').count();
const draftTabs = await page.locator('.draft-tabs .tab').count();

// Phase 1: model + flow survive reload
await page.reload({ waitUntil: 'networkidle' });
const modelAfter = await modelSelect.inputValue();
const flowAfter = await flowSelect.inputValue();
const tabsAfter = await page.locator('.draft-tabs .tab').count();
const svgAfter = await page.locator('#svg-container svg').count();

const result = {
  chosenModel,
  targetFlow,
  beforeReload: { svgCount, draftTabs },
  afterReload: { modelAfter, flowAfter, tabsAfter, svgAfter },
};

console.log(JSON.stringify(result, null, 2));

await browser.close();

const ok =
  svgCount > 0 &&
  modelAfter === chosenModel &&
  flowAfter === targetFlow &&
  tabsAfter > 0 &&
  svgAfter > 0;

process.exit(ok ? 0 : 1);
