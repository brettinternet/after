import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";

// Check repository-local links, never fetch research links or execute HTML.
let checked = 0;
const files = [
  "README.md",
  "AGENTS.md",
  ...Array.from(new Bun.Glob("docs/**/*.md").scanSync(".")),
  ...Array.from(new Bun.Glob("docs/**/*.html").scanSync(".")),
];
for (const file of files) {
  const text = await Bun.file(file).text();
  const pattern = file.endsWith(".html")
    ? /href=["']([^"']+)["']/g
    : /\[[^\]]*\]\(([^\s)]+)(?:\s+"[^"]*")?\)/g;
  for (const match of text.matchAll(pattern)) {
    const link = match[1];
    if (/^(?:[a-z][a-z0-9+.-]*:|#|\/\/)/i.test(link)) continue;
    const target = decodeURIComponent(link.split(/[?#]/)[0]);
    assert.ok(existsSync(resolve(dirname(file), target)), `${file}: broken link ${link}`);
    checked++;
  }
}
console.log(`Checked ${checked} local links in ${files.length} files.`);
