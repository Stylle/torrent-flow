export const KB = 1000,
  MB = 1000000,
  GB = 1000000000;
export function formatBytes(value = 0, speed = false) {
  const v = Number(value) || 0;
  if (!v) return speed ? "0 KB/s" : "0 B";
  const exponent = Math.max(
    speed ? 1 : 0,
    Math.min(4, Math.floor(Math.log(Math.abs(v)) / Math.log(1000))),
  );
  const units = ["B", "KB", "MB", "GB", "TB"];
  const text = (v / 1000 ** exponent).toFixed(3).replace(/\.?0+$/, "");
  return `${text} ${units[exponent]}${speed ? "/s" : ""}`;
}
export const rateInput = (bytes) => Number(bytes || 0) / KB;
export const rateBytes = (input) => Math.round(Number(input || 0) * KB);
