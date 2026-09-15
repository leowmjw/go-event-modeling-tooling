import { chromium } from 'playwright';

// Regression: pick a flow, assert the diagram renders, switch the lens,
// select a step by clicking the SVG, add a staging step through the form,
// then reload and assert model/flow/lens selection persisted.
const url = 'http://localhost:8080/';

const browser = await chromium.launch();
const page = await browser.newPage();
const consoleErrors = [];
page.on('console', (m) => { if (m.type() === 'error') consoleErrors.push(m.text()); });

await page.goto(url, { waitUntil: 'networkidle' });
await page.waitForFunction(() => document.querySelector('form.picker select'));

const modelSelect = page.locator('header .picker-model select');
let chosenModel = await modelSelect.inputValue();
if (!(await modelSelect.isDisabled())) {
  for (const opt of await modelSelect.locator('option').all()) {
    const v = await opt.getAttribute('value');
    if (v && v !== chosenModel) { chosenModel = v; await modelSelect.selectOption(v); break; }
  }
  await page.waitForTimeout(400);
}

const flowSelect = page.locator('form.picker select');
const targetFlow = 'staging-lens-hotspots';
await flowSelect.selectOption(targetFlow);
await page.locator('form.picker button[type=submit]').click();
await page.waitForSelector('.draft-tabs', { timeout: 10000 });
await page.waitForTimeout(800);

const svgCount = await page.locator('#svg-container svg').count();
const stagingBoxesAll = await page.locator('#svg-container .box-stage-staging').count();

// Lens: as-is should hide staging frames.
await page.locator('.lens-btn', { hasText: 'As-is' }).click();
await page.waitForTimeout(800);
const stagingBoxesAsIs = await page.locator('#svg-container .box-stage-staging').count();
await page.locator('.lens-btn', { hasText: 'Future' }).click();
await page.waitForTimeout(800);

// Click a frame in the SVG → inspector opens for it.
await page.locator('#svg-container [data-frame="02"]').click();
await page.waitForTimeout(300);
const inspectorVisible = await page.locator('.inspector:visible h4', { hasText: '02 ·' }).count();

// Add a step through the form.
const stepsBefore = await page.locator('.step-list .step').count();
await page.locator('.panel-head button', { hasText: 'Add step' }).first().click();
await page.locator('form.form:visible select[data-bind\\:step-type]').selectOption('cmd');
await page.locator('form.form:visible input[data-bind\\:step-name]').fill('Review flagged top up');
await page.locator('form.form:visible select[data-bind\\:step-after]').selectOption('09');
await page.locator('form.form:visible button[type=submit]').click();
await page.waitForTimeout(1000);
const stepsAfter = await page.locator('.step-list .step').count();
const newStepStaging = await page.locator('#svg-container [data-frame="13"].box-stage-staging').count();

// Reload: model + flow + lens survive.
await page.reload({ waitUntil: 'networkidle' });
const modelAfter = await modelSelect.inputValue();
const flowAfter = await flowSelect.inputValue();
const lensAfter = await page.locator('.lens-btn.active').innerText();
const svgAfter = await page.locator('#svg-container svg').count();

const result = {
  chosenModel, targetFlow,
  svgCount, stagingBoxesAll, stagingBoxesAsIs, inspectorVisible,
  stepsBefore, stepsAfter, newStepStaging,
  afterReload: { modelAfter, flowAfter, lensAfter, svgAfter },
  consoleErrors,
};
console.log(JSON.stringify(result, null, 2));
await browser.close();

const ok =
  svgCount > 0 &&
  stagingBoxesAll > 0 && stagingBoxesAsIs === 0 &&
  inspectorVisible === 1 &&
  stepsAfter === stepsBefore + 1 && newStepStaging === 1 &&
  modelAfter === chosenModel && flowAfter === targetFlow && lensAfter.startsWith('+ Future') &&
  svgAfter > 0 && consoleErrors.length === 0;
process.exit(ok ? 0 : 1);
