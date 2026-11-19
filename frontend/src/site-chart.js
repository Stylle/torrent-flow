const palette = [
  "#229b67",
  "#3984d6",
  "#d99428",
  "#9b63ce",
  "#d25b6f",
  "#21a0ab",
  "#bc7045",
  "#698c35",
  "#6978c7",
  "#b557a5",
  "#6e8b92",
  "#ba8c39",
];
export function siteColor(id, sites = []) {
  const index = sites.findIndex((s) => s.id === id);
  if (index >= 0)
    return index < palette.length
      ? palette[index]
      : `hsl(${(index * 137.508) % 360},65%,46%)`;
  let hash = 2166136261;
  for (const c of String(id)) {
    hash = Math.imul(hash ^ c.charCodeAt(0), 16777619);
  }
  return `hsl(${(hash >>> 0) % 360}, 65%, 46%)`;
}
export function buildSiteSeries(
  rows,
  sites,
  { bars = false, metric = "up", sampleStep = 15 } = {},
) {
  const names = new Map(sites.map((s) => [s.id, s.name]));
  const observed = [...new Set(rows.map((r) => r.time))].sort((a, b) => a - b);
  const times = [];
  for (const time of observed) {
    const previous = times.at(-1);
    if (!bars && previous !== undefined && time - previous > sampleStep * 1.5)
      times.push(previous + sampleStep);
    times.push(time);
  }
  const ids = [...new Set(rows.map((r) => r.site))].sort();
  const values = new Map();
  for (const row of rows) {
    const key = row.site + ":" + row.time;
    values.set(key, (values.get(key) || 0) + Number(row[metric] || 0));
  }
  return {
    times,
    datasets: ids.map((id) => ({
      id,
      label: names.get(id) || "历史归属",
      color: siteColor(id, sites),
      data: times.map((time) =>
        values.has(id + ":" + time)
          ? values.get(id + ":" + time)
          : bars
            ? 0
            : null,
      ),
    })),
  };
}
