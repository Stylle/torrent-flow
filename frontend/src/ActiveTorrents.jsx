import React from "react";
import { ArrowDown, ArrowUp } from "./icons.jsx";
import { formatBytes } from "./units.js";

const filters = [
  ["all", "count", "全部", "All"],
  ["downloading", "downloading", "下载中", "Downloading"],
  ["uploading", "uploading", "上传中", "Uploading"],
];

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

export function ActiveTorrents({ summary, rows, total, page, filter, changeFilter, changePage, clients, siteName, guard, editTorrent, t }) {
  if (!summary) {
    const demo = demoActivity(rows);
    summary = demo.summary;
    rows = demo.rows;
  }
  if (filter !== "all") rows = rows.filter((row) => row[filter]);
  total ??= rows.length;
  const last = Math.max(0, Math.ceil(total / 50) - 1);
  const downloadLabel = t("下载速度", "Download rate");
  const uploadLabel = t("上传速度", "Upload rate");
  const limitLabel = t("分配限速", "Allocated limit");
  const progressLabel = t("下载进度", "Progress");
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
          {guard}
        </div>
      </div>
      <div className="group activity-filters" role="group" aria-label={t("筛选活跃种子", "Filter active torrents")}>
        {filters.map(([value, key, zh, en]) => (
          <button key={value} type="button" aria-pressed={filter === value}
            className={filter === value ? "active" : ""} onClick={() => changeFilter(value)}>
            {t(zh, en)} <span>{summary[key]}</span>
          </button>
        ))}
      </div>
      {rows.length === 0 ? (
        <div className="empty">{t("暂无符合条件的活跃种子", "No matching active torrents")}</div>
      ) : (
        <div className="table-wrap activity-list" key={filter + ":" + page}>
          <table className="activity-table" role="table">
            <colgroup>
              <col className="activity-name-col" />
              <col className="activity-progress-col" />
              <col />
              <col />
              <col />
              <col className="activity-peers-col" />
              <col className="activity-status-col" />
            </colgroup>
            <thead role="rowgroup"><tr role="row">
              {[t("种子名称", "Torrent name"), progressLabel, downloadLabel, uploadLabel, limitLabel, connectionsLabel, t("状态", "Status")].map((label) =>
                <th key={label} scope="col">{label}</th>
              )}
            </tr></thead>
            <tbody role="rowgroup">{rows.map((row) => {
              const progress = typeof row.progress === "number" ? Math.min(1, Math.max(0, row.progress)) : null;
              const source = [clientNames.get(row.client) || row.client, siteName(row.site)].filter(Boolean).join(" · ");
              const status = uploadStatus(row, t);
              return (
                <tr key={row.key} role="row">
                  <td className="activity-name">
                    <div className="activity-identity">
                    <button type="button" onClick={() => editTorrent(row)} title={row.name}>{row.name}</button>
                    <small title={[source, ...(row.hosts || [])].join(" · ")}>{source}</small>
                    </div>
                  </td>
                  <ActivityValue label={progressLabel} className="activity-progress">
                    {progress === null ? "—" : (
                      <div className="download-progress">
                        <progress max="1" value={progress} aria-label={progressLabel} />
                        <span>{(progress * 100).toLocaleString(undefined, { maximumFractionDigits: 1 })}%</span>
                      </div>
                    )}
                  </ActivityValue>
                  <ActivityValue label={downloadLabel} className="activity-download">{rate(row.down)}</ActivityValue>
                  <ActivityValue label={uploadLabel} className="activity-upload">{rate(row.up)}</ActivityValue>
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
      {total > 0 && (
        <div className="activity-footer">
          <small>{t(`共 ${total} 个种子`, `${total} torrents`)}</small>
          {last > 0 && <div className="pagination">
            <button type="button" disabled={page === 0} onClick={() => changePage(page - 1)}>{t("上一页", "Previous")}</button>
            <small>{page + 1} / {last + 1}</small>
            <button type="button" disabled={page >= last} onClick={() => changePage(page + 1)}>{t("下一页", "Next")}</button>
          </div>}
        </div>
      )}
    </section>
  );
}
