import React, { useLayoutEffect, useRef, useState } from "react";
import { ArrowDown, ArrowUp } from "./icons.jsx";
import { formatBytes } from "./units.js";
import { siteColor } from "./site-chart.js";
import { ColumnResizeHandle } from "./ColumnResizeHandle.jsx";

const filters = [
  ["all", "count", "全部", "All"],
  ["downloading", "downloading", "下载中", "Downloading"],
  ["uploading", "uploading", "上传中", "Uploading"],
];

const minimumWidths = [180, 128, 105, 96, 96, 85, 100, 87, 105];
const minimumTotal = minimumWidths.reduce((sum, width) => sum + width, 0);
const defaultRatios = [6500, 950, 550, 350, 350, 450, 350, 150, 350];

function resizePair(ratios, index, delta) {
  const next = [...ratios];
  const combined = next[index] + next[index + 1];
  next[index] = Math.max(0, Math.min(combined, next[index] + delta));
  next[index + 1] = combined - next[index];
  return next;
}

function demoActivity(torrents) {
  const rows = torrents
    .map((row) => ({ ...row, uploading: row.up > 0 || row.connections > 0 }))
    .filter((row) => row.downloading || row.uploading);
  return {
    rows,
    summary: rows.reduce((stats, row) => ({
      count: stats.count + 1,
      downloading: stats.downloading + Number(row.downloading),
      uploading: stats.uploading + Number(row.uploading),
      up: stats.up + row.up,
      down: stats.down + row.down,
    }), { count: 0, downloading: 0, uploading: 0, up: 0, down: 0 }),
  };
}

function uploadStatus(row, t) {
  if (row.managed) return ["allocated", t("已分配", "Allocated")];
  if (row.state === "release pending") return ["warning", t("恢复待重试", "Recovery pending")];
  if (row.state === "external limit change") return ["warning", t("外部限速变更", "External limit changed")];
  if (row.awaitingUpload) return ["warning", t("待确认", "Confirming")];
  if (row.state && !["allocated", "monitoring", "released"].includes(row.state))
    return ["warning", t("分配失败", "Allocation failed")];
  if (row.uploading) return ["neutral", t("上传监控中", "Monitoring upload")];
  return null;
}

function ActivityValue({ label, className, children }) {
  return (
    <td className={className}>
      <span className="activity-field-label" aria-hidden="true">{label}</span>
      {children}
    </td>
  );
}

export function ActiveTorrents({ summary, rows, total, page, filter, changeFilter, changePage, clients, sites, siteName, allocated, guard, editTorrent, columnRatios, saveColumnRatios, t }) {
  const table = useRef(null);
  const list = useRef(null);
  const drag = useRef(null);
  const [tableWidth, setTableWidth] = useState(0);
  const [preview, setPreview] = useState(null);
  const [boundary, setBoundary] = useState(null);
  const [saving, setSaving] = useState(false);
  const ratios = preview ?? (columnRatios?.length === minimumWidths.length ? columnRatios : defaultRatios);
  const widthFor = (minimum, ratio) => minimum + Math.max(0, tableWidth - minimumTotal) * ratio / 10000;
  async function persist(next) {
    setSaving(true);
    setPreview(next);
    try {
      await saveColumnRatios(next);
    } finally {
      setPreview(null);
      setSaving(false);
    }
  }
  if (!summary) {
    const demo = demoActivity(rows);
    summary = demo.summary;
    rows = demo.rows;
    allocated = rows.reduce((sum, row) => sum + (row.managed ? row.limit : 0), 0);
  }
  if (filter !== "all") rows = rows.filter((row) => row[filter]);
  const hasRows = rows.length > 0;
  useLayoutEffect(() => {
    if (!hasRows) return;
    const element = list.current;
    const measure = () => setTableWidth(element.clientWidth);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [hasRows, filter, page]);
  total ??= rows.length;
  const last = Math.max(0, Math.ceil(total / 50) - 1);
  const downloadLabel = t("下载速度", "Download rate");
  const uploadLabel = t("上传速度", "Upload rate");
  const limitLabel = t("分配限速", "Allocated limit");
  const progressLabel = t("下载进度", "Progress");
  const downloaderLabel = t("下载器", "Downloader");
  const siteLabel = t("站点", "Site");
  const connectionsLabel = t("上传连接", "Upload peers");
  const clientNames = new Map(clients.map((client) => [client.id, client.name]));
  const rate = (value) => value > 0 ? formatBytes(value, true) : "—";
  return (
    <section className="card active-torrents" aria-labelledby="active-torrents-title">
      <div className="activity-header">
        <h2 id="active-torrents-title">{t("活跃种子", "Active torrents")}</h2>
        <div className="activity-rates">
          <span title={downloadLabel}><ArrowDown className="down" size={17} />{formatBytes(summary.down, true)}</span>
          <span title={uploadLabel}><ArrowUp className="up" size={17} />{formatBytes(summary.up, true)}</span>
          {allocated != null && <span className="activity-allocated">
            {t("已分配带宽", "Allocated bandwidth")} <strong>{formatBytes(allocated, true)}</strong>
          </span>}
        </div>
        <div className="group activity-filters" role="group" aria-label={t("筛选活跃种子", "Filter active torrents")}>
        {filters.map(([value, key, zh, en]) => (
          <button key={value} type="button" aria-pressed={filter === value}
            className={filter === value ? "active" : ""} onClick={() => changeFilter(value)}>
            {t(zh, en)} <span>{summary[key]}</span>
          </button>
        ))}
        </div>
      </div>
      {rows.length === 0 ? (
        <div className="empty">{t("暂无符合条件的活跃种子", "No matching active torrents")}</div>
      ) : (
        <div className="table-wrap activity-list" key={filter + ":" + page} ref={list}>
          {boundary !== null && <div className="activity-resize-guide" aria-hidden="true"
            style={{ left: widthFor(minimumWidths.slice(0, boundary + 1).reduce((sum, width) => sum + width, 0),
              ratios.slice(0, boundary + 1).reduce((sum, ratio) => sum + ratio, 0)) }} />}
          <table className="activity-table" role="table" ref={table}>
            <colgroup>
              {minimumWidths.map((width, index) => <col key={index} style={{ width: widthFor(width, ratios[index]) }} />)}
            </colgroup>
            <thead role="rowgroup"><tr role="row">
              {[t("种子名称", "Torrent name"), progressLabel, downloaderLabel, downloadLabel, uploadLabel, siteLabel, limitLabel, connectionsLabel, t("状态", "Status")].map((label, index) =>
                <th key={label} scope="col">
                  {label}
                  {index < minimumWidths.length - 1 && (
                    <ColumnResizeHandle
                      label={t("调整列宽：", "Resize column: ") + label}
                      min={0} max={ratios[index] + ratios[index + 1]} value={ratios[index]}
                      active={boundary === index}
                      start={() => {
                        const extra = table.current.clientWidth - minimumTotal;
                        if (saving || drag.current || extra <= 0) return false;
                        drag.current = { ratios, extra, next: ratios };
                        setBoundary(index);
                      }}
                      move={(delta) => {
                        const current = drag.current;
                        current.next = resizePair(current.ratios, index, Math.round(delta * 10000 / current.extra));
                        setPreview(current.next);
                      }}
                      finish={(commit) => {
                        const { next, ratios: start } = drag.current;
                        drag.current = null;
                        setBoundary(null);
                        if (commit && next[index] !== start[index]) void persist(next);
                        else setPreview(null);
                      }}
                      step={(direction) => {
                        if (saving || drag.current) return;
                        const next = resizePair(ratios, index, direction * 200);
                        if (next[index] !== ratios[index]) void persist(next);
                      }}
                    />
                  )}
                </th>
              )}
            </tr></thead>
            <tbody role="rowgroup">{rows.map((row) => {
              const progress = typeof row.progress === "number" ? Math.min(1, Math.max(0, row.progress)) : null;
              const downloader = clientNames.get(row.client) || row.client;
              const site = siteName(row.site);
              const status = uploadStatus(row, t);
              return (
                <tr key={row.key} role="row">
                  <td className="activity-name">
                    <button type="button" onClick={() => editTorrent(row)} title={row.name}>{row.name}</button>
                  </td>
                  <ActivityValue label={progressLabel} className="activity-progress">
                    {progress === null ? "—" : (
                      <div className="download-progress">
                        <span>{(progress * 100).toLocaleString(undefined, { maximumFractionDigits: 1 })}%</span>
                        <progress max="1" value={progress} aria-label={progressLabel} />
                      </div>
                    )}
                  </ActivityValue>
                  <ActivityValue label={downloaderLabel}>
                    <span className="activity-cell-text activity-downloader" title={downloader}>{downloader || "—"}</span>
                  </ActivityValue>
                  <ActivityValue label={downloadLabel} className={row.down > 0 ? "activity-download has-rate" : "activity-download"}>{rate(row.down)}</ActivityValue>
                  <ActivityValue label={uploadLabel} className={row.up > 0 ? "activity-upload has-rate" : "activity-upload"}>{rate(row.up)}</ActivityValue>
                  <ActivityValue label={siteLabel}>
                    <span className={site ? "activity-cell-text activity-site" : "activity-cell-text"}
                      style={site ? { "--activity-site-color": siteColor(row.site, sites) } : undefined}
                      title={[site, ...(row.hosts || [])].filter(Boolean).join(" · ")}>
                      {site && <i aria-hidden="true" />}
                      {site || "—"}
                    </span>
                  </ActivityValue>
                  <ActivityValue label={limitLabel} className="activity-limit">{row.managed ? rate(row.limit) : "—"}</ActivityValue>
                  <ActivityValue label={connectionsLabel} className="activity-peers">{row.connections}</ActivityValue>
                  <td className={`activity-status ${status?.[0] || (row.down > 0 ? "downloading" : "neutral")}`} title={row.state}>
                    {row.downloading && <span className={row.down > 0 ? undefined : "muted"}>
                      {row.down > 0 ? t("下载中", "Downloading") : t("等待数据", "Waiting for data")}
                    </span>}
                    {row.downloading && status && <span aria-hidden="true"> · </span>}
                    {status && <span className={status[0]}>{status[1]}</span>}
                  </td>
                </tr>
              );
            })}</tbody>
          </table>
        </div>
      )}
      {(total > 0 || guard) && (
        <div className="activity-footer">
          {total > 0 && <small>{t(`共 ${total} 个种子`, `${total} torrents`)}</small>}
          {last > 0 && <div className="pagination">
            <button type="button" disabled={page === 0} onClick={() => changePage(page - 1)}>{t("上一页", "Previous")}</button>
            <small>{page + 1} / {last + 1}</small>
            <button type="button" disabled={page >= last} onClick={() => changePage(page + 1)}>{t("下一页", "Next")}</button>
          </div>}
          {guard}
        </div>
      )}
    </section>
  );
}
