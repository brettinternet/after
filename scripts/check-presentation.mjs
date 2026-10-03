import { chromium } from "playwright";
import assert from "node:assert/strict";
import { writeFile, readFile, mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
const out = resolve("artifacts/presentation");
await mkdir(out, { recursive: true });
const artifacts = process.argv.includes("--render") ? resolve("docs") : out;
const url = pathToFileURL(resolve("docs/presentation.html")).href;
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({
  viewport: { width: 1440, height: 900 },
  reducedMotion: "reduce",
});
const errors = [];
const network = [];
page.on("pageerror", (error) => errors.push(String(error)));
page.on("request", (request) => {
  if (/^https?:/.test(request.url())) network.push(request.url());
});
async function slide(n) {
  await page.goto(`${url}#${n}`);
  await page.locator(`#slide-${n}`).waitFor({ state: "visible" });
}
const layouts = [];
try {
  for (let n = 1; n <= 12; n++) {
    await slide(n);
    assert.equal(await page.locator(".slide:visible").count(), 1);
    layouts.push(
      await page.locator(`#slide-${n}`).evaluate((el) => ({
        id: el.id,
        width: el.clientWidth,
        scrollWidth: el.scrollWidth,
        height: el.clientHeight,
        scrollHeight: el.scrollHeight,
        firstTop: el.firstElementChild.getBoundingClientRect().top,
        top: el.getBoundingClientRect().top,
      })),
    );
    const layout = layouts.at(-1);
    assert.ok(layout.scrollWidth <= layout.width, `desktop horizontal overflow: ${layout.id}`);
    assert.ok(layout.scrollHeight <= layout.height, `desktop vertical overflow: ${layout.id}`);
    await page.screenshot({ path: `${out}/slide-${n}.png` });
  }
  await slide(4);
  assert.equal(await page.locator("#edit-btn").isDisabled(), true);
  await page.click("#pin-btn");
  assert.match(await page.locator("#pin-badge").innerText(), /Pinned/);
  await page.click("#edit-btn");
  assert.equal(await page.locator("#latest-value").innerText(), "Not run");
  assert.match(await page.locator("#demo-status").innerText(), /Nothing has run/);
  await page.click("#tab-outcomes");
  assert.equal(await page.locator("#row-mid").innerText(), "Not run");
  await page.click("#run-btn");
  assert.equal(await page.locator("#row-mid").innerText(), "2 charges");
  assert.equal(await page.locator("#row-fast").innerText(), "Not rerun");
  assert.equal(await page.locator("#row-expired").innerText(), "Not rerun");
  await page.click("#tab-memory");
  assert.match(await page.locator("#memory-title").innerText(), /disagree/);
  await page.keyboard.press("ArrowLeft");
  assert.equal(await page.locator("#tab-cause").getAttribute("aria-selected"), "true");
  assert.equal(await page.locator("#slide-4").isVisible(), true);
  await page.click("#tab-glance");
  await page.screenshot({ path: `${out}/demo-after.png` });
  await page.click("#inventory-open");
  assert.equal(await page.locator("#inventory-dialog").isVisible(), true);
  await page.keyboard.press("Escape");
  assert.equal(await page.locator("#inventory-dialog").isVisible(), false);
  await page.click("#notes-btn");
  assert.equal(await page.locator("#speaker-notes").isVisible(), true);
  await page.click("#notes-close");
  await page.click("#reset-btn");
  assert.equal(await page.locator("#latest-value").innerText(), "1 charge");
  assert.equal(await page.locator("#run-btn").isDisabled(), true);
  await slide(5);
  for (const [key, value] of [
    ["fast", "1"],
    ["middle", "2"],
    ["expired", "2"],
    ["unknown", "?"],
  ]) {
    await page.click(`[data-delay="${key}"]`);
    assert.equal(await page.locator("#boundary-after").innerText(), value);
  }
  assert.equal(await page.locator("#boundary-badge").innerText(), "Not run");
  await slide(1);
  await page.keyboard.press("ArrowRight");
  await page.locator("#slide-2").waitFor({ state: "visible" });
  assert.equal(await page.locator("#slide-2").isVisible(), true);
  await page.keyboard.press("End");
  await page.locator("#slide-12").waitFor({ state: "visible" });
  assert.equal(await page.locator("#slide-12").isVisible(), true);
  const mobile = [];
  await page.setViewportSize({ width: 390, height: 844 });
  for (let n = 1; n <= 12; n++) {
    await slide(n);
    const layout = await page.evaluate(() => ({
      width: innerWidth,
      scrollWidth: document.documentElement.scrollWidth,
      bodyWidth: document.body.scrollWidth,
    }));
    mobile.push({ slide: n, ...layout });
    assert.ok(
      layout.scrollWidth <= layout.width,
      `mobile horizontal overflow slide ${n}: ${JSON.stringify(layout)}`,
    );
  }
  await slide(4);
  await page.screenshot({ path: `${out}/mobile.png`, fullPage: true });
  await page.setViewportSize({ width: 1440, height: 900 });
  await slide(1);
  await page.screenshot({ path: `${artifacts}/preview.png` });
  await page.emulateMedia({ media: "print" });
  const printLayouts = await page.locator(".slide").evaluateAll((els) =>
    els.map((el) => ({
      id: el.id,
      height: el.clientHeight,
      scrollHeight: el.scrollHeight,
      width: el.clientWidth,
      scrollWidth: el.scrollWidth,
    })),
  );
  for (const layout of printLayouts) {
    assert.ok(layout.scrollHeight <= layout.height + 1, `print vertical clipping: ${layout.id}`);
    assert.ok(layout.scrollWidth <= layout.width + 1, `print horizontal clipping: ${layout.id}`);
  }
  await page.pdf({
    path: `${artifacts}/presentation.pdf`,
    printBackground: true,
    preferCSSPageSize: true,
    displayHeaderFooter: false,
  });
  const pdf = await readFile(`${artifacts}/presentation.pdf`);
  const pages = (pdf.toString("latin1").match(/\/Type\s*\/Page\b/g) || []).length;
  assert.equal(pages, 12, "PDF must contain exactly twelve pages");
  assert.equal(errors.length, 0, `page errors: ${errors.join("\n")}`);
  assert.equal(network.length, 0, "presentation must be network-free");
  const result = {
    passed: true,
    pdfPages: pages,
    errors,
    network,
    desktop: layouts,
    mobile,
    print: printLayouts,
  };
  await writeFile(`${out}/results.json`, JSON.stringify(result, null, 2));
  console.log(JSON.stringify(result, null, 2));
} finally {
  await browser.close();
}
