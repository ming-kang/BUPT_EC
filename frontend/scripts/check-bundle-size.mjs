import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

// Budget = ceil(measured total × 1.10) (headroom for dependency patch churn).
// Measurement basis (this script's own metric: gzip level 9 over every
// js/css/html/svg file in dist/ — do NOT compare against vite's console
// numbers, which use zlib's default compression level):
//   2026-07-27 measured total = 209,898 B
//   (antd 5.29.3 / react 18.3.1 / vite 7.3.6, react-vendor + antd-vendor
//   manual chunks, native table, inline icons)
// Pre-upgrade baseline for reference: 293,407 B (2026-07-27, antd 5.12.6 /
// vite 6.4.3, same gzip -9 metric). If you exceed the budget intentionally,
// update BUDGET_BYTES here with a justification.
//
// Known and accepted trade-off: this is a TOTAL budget, not a first-load one.
// Grouping antd into one eagerly preloaded vendor chunk hoists the parts that
// used to live in lazy chunks (Modal/Empty/Tag/Divider/Switch + rc-dialog),
// so first load went 183,599 B → 207,242 B (index.html + index js/css +
// react-vendor + antd-vendor) even though the total dropped ~28%. Bought with
// it: antd-vendor keeps its hash across application-only releases. Revisit
// (per-entry budget, or a getModuleInfo-based "statically reachable only"
// split) if antd usage in lazy routes grows.
const BUDGET_BYTES = 230_888;

const dist = join(fileURLToPath(new URL('..', import.meta.url)), 'dist');
const files = [];
(function walk(dir) {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) {
      walk(p);
    } else if (/\.(js|css|html|svg)$/.test(e.name)) {
      files.push(p);
    }
  }
})(dist);

let total = 0;
for (const f of files.sort()) {
  const gz = gzipSync(readFileSync(f), { level: 9 }).length;
  total += gz;
  console.log(`${String(gz).padStart(9)} B gzip  ${f.slice(dist.length + 1)}`);
}
console.log(`total ${total} B gzip (budget ${BUDGET_BYTES} B)`);
if (total > BUDGET_BYTES) {
  console.error(
    `Bundle size budget exceeded by ${total - BUDGET_BYTES} B. ` +
      'If intentional, update BUDGET_BYTES with justification.',
  );
  process.exit(1);
}
