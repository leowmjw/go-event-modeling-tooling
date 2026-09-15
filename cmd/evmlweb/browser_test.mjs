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
// Workshop-friendly UX surfaces from the 2026-09 overhaul:
const hotspotItems = await page.locator('.hotspot-item').count();
const chapterItems = await page.locator('.chapter-item').count();
const sliceItems = await page.locator('.slice-item').count();
const hotspotPins = await page.locator('.hotspot-pin').count();

// Phase 1: model + flow survive reload
await page.reload({ waitUntil: 'networkidle' });
const modelAfter = await modelSelect.inputValue();
const flowAfter = await flowSelect.inputValue();
const tabsAfter = await page.locator('.draft-tabs .tab').count();
const svgAfter = await page.locator('#svg-container svg').count();

// Phase 2: open a FinTech fixture and confirm new UX surfaces appear.
const finTechFlows = [
  'card-payment-lifecycle',
  'retail-account-onboarding-kyc',
  'unsecured-loan-origination',
  'fraud-aml-real-time',
];
let finTechProbe = null;
for (const candidate of finTechFlows) {
  const exists = await flowSelect.locator(`option[value="${candidate}"]`).count();
  if (exists > 0) {
    await flowSelect.selectOption(candidate);
    await page.locator('form.picker button[type=submit]').click();
    await page.waitForTimeout(800);
    finTechProbe = {
      flow: candidate,
      hotspots: await page.locator('.hotspot-item').count(),
      chapters: await page.locator('.chapter-item').count(),
      slices: await page.locator('.slice-item').count(),
      pins: await page.locator('.hotspot-pin').count(),
    };
    break;
  }
}

const result = {
  chosenModel,
  targetFlow,
  baseline: {
    svgCount, draftTabs,
    hotspotItems, chapterItems, sliceItems, hotspotPins,
  },
  afterReload: { modelAfter, flowAfter, tabsAfter, svgAfter },
  finTechProbe,
};

console.log(JSON.stringify(result, null, 2));

await browser.close();

const baselineOK =
  svgCount > 0 &&
  modelAfter === chosenModel &&
  flowAfter === targetFlow &&
  tabsAfter > 0 &&
  svgAfter > 0;

// The FinTech check is only meaningful when at least one of the four
// candidate fixtures is present; downstream forks may rename them, so
// we degrade to "test ran" when none are found.
const finTechOK =
  finTechProbe === null ||
  (finTechProbe.hotspots > 0 &&
    finTechProbe.chapters > 0 &&
    finTechProbe.slices > 0);

process.exit(baselineOK && finTechOK ? 0 : 1);
