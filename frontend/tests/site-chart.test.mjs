import test from "node:test";
import assert from "node:assert/strict";
import { buildSiteSeries, siteColor } from "../src/site-chart.js";
test("stacked bars preserve 50/10/40 bytes and site identity", () => {
  const sites = [
      { id: "a", name: "A" },
      { id: "b", name: "B" },
      { id: "c", name: "C" },
    ],
    rows = [
      { time: 1, site: "a", up: 50 },
      { time: 1, site: "b", up: 10 },
      { time: 1, site: "c", up: 40 },
    ];
  const result = buildSiteSeries(rows, sites, { bars: true });
  assert.deepEqual(
    result.datasets.map((d) => d.data[0]),
    [50, 10, 40],
  );
  assert.equal(
    result.datasets.reduce((s, d) => s + d.data[0], 0),
    100,
  );
  assert.equal(result.datasets[0].color, siteColor("a", sites));
  assert.equal(
    buildSiteSeries(
      rows,
      sites.map((s) => (s.id === "a" ? { ...s, name: "Renamed" } : s)),
      { bars: true },
    ).datasets[0].color,
    siteColor("a", sites),
  );
});
test("missing line samples remain gaps rather than invented zeroes", () => {
  const r = buildSiteSeries(
    [
      { time: 1, site: "a", up: 10 },
      { time: 2, site: "b", up: 20 },
    ],
    [],
    {},
  );
  assert.deepEqual(r.datasets[0].data, [10, null]);
  assert.deepEqual(r.datasets[1].data, [null, 20]);
});

test("a global offline interval breaks every line", () => {
  const r = buildSiteSeries(
    [
      { time: 100, site: "a", up: 10 },
      { time: 160, site: "a", up: 20 },
    ],
    [{ id: "a", name: "A" }],
  );
  assert.deepEqual(r.times, [100, 115, 160]);
  assert.deepEqual(r.datasets[0].data, [10, null, 20]);
});
