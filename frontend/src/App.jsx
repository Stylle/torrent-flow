import React, { useState, useEffect, useRef, useMemo } from "react";
import {
  WaveSine,
  House,
  ChartBar,
  Stack,
  DownloadSimple,
  GearSix,
  Sun,
  Moon,
  Desktop,
  List,
  X,
  SignOut,
  Plus,
  ArrowUp,
  ArrowDown,
  Broadcast,
  ShieldCheck,
  Info,
  ArrowClockwise,
  Trash,
  Link,
  FloppyDisk,
  Globe,
  ArrowRight,
} from "./icons.jsx";
import Chart from "chart.js/auto";
import "./app.css";
import { createID } from "./identity.js";
import { buildSiteSeries, siteColor } from "./site-chart.js";
import { MB, GB, formatBytes as bytes, rateInput, rateBytes } from "./units.js";
const id = createID;
function demoData() {
  const now = Date.now() / 1000,
    sites = ["北辰站", "海湾站", "山海站"].map((name, i) => ({
      id: "s" + i,
      name,
      domains: [
        ["a.example", "b.example"],
        ["bay.example"],
        ["mountain.example"],
      ][i],
      regex: "",
      weight: [5, 3, 1][i],
      minimum: MB,
      maximum: 20 * MB,
      dayTarget: 0,
      monthTarget: 0,
      afterTarget: MB,
    }));
  return {
    version: "0.1.13",
    catalog: {
      sites: 329,
      domains: 780,
      version: "demo",
      auto: true,
      updated: Math.floor(now),
    },
    downloaderStats: [
      { id: "qb", up: 102 * GB, down: 28 * GB },
      { id: "tr", up: 63 * GB, down: 15 * GB },
    ],
    start: new Date(new Date().setHours(0, 0, 0, 0)).getTime() / 1000,
    end: new Date(new Date().setHours(24, 0, 0, 0)).getTime() / 1000,
    config: {
      revision: 0,
      timezone: "Asia/Shanghai",
      pollSeconds: 5,
      idleSeconds: 15,
      uploadConfirmSeconds: 0,
      sites,
      pools: [{ id: "home", name: "家庭网络", budget: 30 * MB, enabled: true }],
      clients: [
        {
          id: "qb",
          name: "qBittorrent NAS",
          kind: "qb",
          url: "http://nas.example:8080",
          user: "admin",
          pool: "home",
          enabled: true,
        },
        {
          id: "tr",
          name: "Transmission VPS",
          kind: "tr",
          url: "http://vps.example:9091/transmission/rpc",
          user: "admin",
          pool: "home",
          enabled: true,
        },
      ],
      overrides: {},
    },
    clients: [
      {
        id: "qb",
        name: "qBittorrent NAS",
        kind: "qb",
        online: true,
        up: 12.1 * MB,
        down: 2.7 * MB,
      },
      {
        id: "tr",
        name: "Transmission VPS",
        kind: "tr",
        online: true,
        up: 6.5 * MB,
        down: 1.8 * MB,
      },
    ],
    torrents: ["City.Walk.2026", "Ocean.Documentary", "Mountain.Light"].map(
      (name, i) => ({
        key: (i === 1 ? "tr:" : "qb:") + i,
        client: i === 1 ? "tr" : "qb",
        name,
        hosts: [
          ["tracker.north.example"],
          ["tracker.bay.example"],
          ["tracker.mountain.example"],
        ][i],
        site: "s" + i,
        connections: [3, 2, 0][i],
        downloading: i === 1,
        progress: i === 1 ? 0.64 : undefined,
        down: i === 1 ? 2.1 * MB : 0,
        limit: [8, 6, 0][i] * MB,
        up: [8.2, 6.4, 4][i] * MB,
        managed: i < 2,
        state: i < 2 ? "allocated" : "released",
      }),
    ),
    buckets: sites.map((s, i) => ({
      site: s.id,
      time: now,
      up: [86.2, 64.1, 36.1][i] * GB,
      down: [12.4, 18.6, 11.8][i] * GB,
    })),
    siteSpeeds: sites.flatMap((site, i) =>
      Array.from({ length: 120 }, (_, j) => ({
        site: site.id,
        time: now - (119 - j) * 15,
        up: (3 + i + Math.sin(j / 7 + i)) * MB,
        down: ((i + 1) * MB) / 4,
      })),
    ),
    speeds: Array.from({ length: 60 }, (_, i) => ({
      time: now - (59 - i) * 30,
      up: (18 + 7 * Math.sin(i / 25) + 2 * Math.sin(i / 3)) * MB,
      down: (5 + 1.7 * Math.sin(i / 7)) * MB,
    })),
    leases: [],
    events: [],
    guardian: true,
  };
}
function Plot({
  data,
  bars = false,
  dark,
  sites,
  metric = "up",
  rangeSeconds = 1800,
}) {
  const ref = useRef();
  const [hidden, setHidden] = useState([]);
  const series = useMemo(
    () =>
      buildSiteSeries(data, sites, {
        bars,
        metric,
        sampleStep: Math.max(15, Math.floor(rangeSeconds / 480)),
      }),
    [data, sites, bars, metric, rangeSeconds],
  );
  useEffect(() => {
    const css = getComputedStyle(document.documentElement),
      muted = css.getPropertyValue("--muted").trim(),
      line = css.getPropertyValue("--line").trim();
    const chart = new Chart(ref.current, {
      type: bars ? "bar" : "line",
      data: {
        labels: series.times.map((time) =>
          new Date(time * 1000).toLocaleString(
            undefined,
            bars && rangeSeconds > 172800
              ? { month: "numeric", day: "numeric" }
              : { hour: "2-digit", minute: "2-digit" },
          ),
        ),
        datasets: series.datasets.map((s) => ({
          label: s.label,
          data: bars
            ? s.data
            : s.data.map((value, i) => ({ x: series.times[i], y: value })),
          borderColor: s.color,
          backgroundColor: s.color,
          borderWidth: bars ? 0 : 2,
          pointRadius: series.times.length === 1 ? 3 : 0,
          tension: 0.15,
          fill: false,
          spanGaps: false,
          hidden: hidden.includes(s.id),
        })),
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        interaction: { mode: "index", intersect: false },
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              title: (items) =>
                bars
                  ? items[0]?.label
                  : new Date((items[0]?.parsed.x || 0) * 1000).toLocaleString(),
              label: (ctx) => {
                const total = ctx.chart.data.datasets.reduce(
                  (n, ds, i) =>
                    n +
                    (ctx.chart.isDatasetVisible(i)
                      ? Number(ds.data[ctx.dataIndex] || 0)
                      : 0),
                  0,
                );
                return `${ctx.dataset.label}: ${bytes(bars ? ctx.raw : ctx.parsed.y, !bars)}${bars && total ? ` (${((Number(ctx.raw) / total) * 100).toFixed(1)}%)` : ""}`;
              },
            },
          },
        },
        scales: {
          x: {
            type: bars ? "category" : "linear",
            stacked: bars,
            grid: { display: false },
            ticks: {
              color: muted,
              maxTicksLimit: 8,
              maxRotation: 0,
              ...(bars
                ? {}
                : {
                    callback: (value) =>
                      new Date(Number(value) * 1000).toLocaleTimeString(
                        undefined,
                        { hour: "2-digit", minute: "2-digit" },
                      ),
                  }),
            },
          },
          y: {
            stacked: bars,
            beginAtZero: true,
            grid: { color: line },
            ticks: { color: muted, callback: (v) => bytes(v, !bars) },
          },
        },
      },
    });
    return () => chart.destroy();
  }, [series, bars, dark, hidden, rangeSeconds]);
  return (
    <div>
      <div className="chart">
        <canvas
          ref={ref}
          role="img"
          aria-label={series.datasets.map((s) => s.label).join(" / ")}
          data-series={series.datasets.length}
          data-stacked={bars}
        />
      </div>
      <div className="chart-legend">
        {series.datasets.map((s) => (
          <button
            key={s.id}
            type="button"
            aria-pressed={!hidden.includes(s.id)}
            className={hidden.includes(s.id) ? "muted" : ""}
            onClick={() =>
              setHidden((v) =>
                v.includes(s.id) ? v.filter((x) => x !== s.id) : [...v, s.id],
              )
            }
          >
            <i style={{ background: s.color }} />
            {s.label}
          </button>
        ))}
      </div>
    </div>
  );
}
function isoWeekValue(day) {
  const d = new Date(day + "T00:00:00Z");
  d.setUTCDate(d.getUTCDate() + 4 - (d.getUTCDay() || 7));
  const year = d.getUTCFullYear();
  const week = Math.ceil(
    ((d - new Date(Date.UTC(year, 0, 1))) / 86400000 + 1) / 7,
  );
  return `${year}-W${String(week).padStart(2, "0")}`;
}
function weekStart(value) {
  const [year, week] = value.split("-W").map(Number);
  const d = new Date(Date.UTC(year, 0, 4));
  d.setUTCDate(d.getUTCDate() - (d.getUTCDay() || 7) + 1 + (week - 1) * 7);
  return d.toISOString().slice(0, 10);
}
export function App() {
  const [diagnosticFilter, setDiagnosticFilter] = useState("");
  const demo = new URLSearchParams(location.search).get("demo") === "1";
  const [data, setData] = useState(demo ? demoData() : null),
    [auth, setAuth] = useState(demo ? "yes" : "loading"),
    [csrf, setCsrf] = useState(""),
    [page, setPage] = useState(0),
    [torrentPage, setTorrentPage] = useState(0),
    [downloadPage, setDownloadPage] = useState(0),
    [menu, setMenu] = useState(false),
    [period, setPeriod] = useState("day"),
    [speedWindow, setSpeedWindow] = useState(1800),
    [speedMetric, setSpeedMetric] = useState("up"),
    [trafficMetric, setTrafficMetric] = useState("up"),
    [columnWidths, setColumnWidths] = useState(() => {
      try {
        const v = JSON.parse(localStorage.getItem("flow.columns.v1"));
        if (
          Array.isArray(v) &&
          v.length === 8 &&
          v.every((n) => Number.isFinite(n) && n >= 70 && n <= 1600)
        )
          return v;
      } catch {}
      return [
        matchMedia("(max-width: 600px)").matches ? 180 : 300,
        115,
        115,
        110,
        155,
        125,
        90,
        205,
      ];
    }),
    [date, setDate] = useState(new Date().toLocaleDateString("en-CA")),
    [lang, setLang] = useState(localStorage.getItem("flow.lang") || "zh"),
    [theme, setTheme] = useState(
      localStorage.getItem("flow.theme") || "system",
    ),
    [dark, setDark] = useState(false),
    [toast, setToast] = useState(""),
    [error, setError] = useState(""),
    [modal, setModal] = useState(null),
    [form, setForm] = useState({}),
    [busy, setBusy] = useState(false),
    [search, setSearch] = useState(""),
    [backupFile, setBackupFile] = useState(null),
    [notificationJobs, setNotificationJobs] = useState([]);
  const backupInput = useRef(null);
  const columnDrag = useRef(null);
  const restoreInFlight = useRef(false);
  const t = (zh, en) => (lang === "zh" ? zh : en),
    c = data?.config;
  useEffect(() => {
    if (page !== 5 || auth !== "yes" || demo) return;
    let active = true;
    api("notification-jobs")
      .then((v) => {
        if (active) setNotificationJobs(v);
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [page, auth, data?.version]);
  useEffect(() => {
    const timer = setTimeout(
      () =>
        localStorage.setItem("flow.columns.v1", JSON.stringify(columnWidths)),
      150,
    );
    return () => clearTimeout(timer);
  }, [columnWidths]);
  useEffect(() => setTorrentPage(0), [page, search]);
  useEffect(() => {
    localStorage.setItem("flow.lang", lang);
    document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
    document.title = lang === "zh" ? "流量台 · Torrent Flow" : "Torrent Flow";
  }, [lang]);
  useEffect(() => {
    localStorage.setItem("flow.theme", theme);
    const mq = matchMedia("(prefers-color-scheme: dark)"),
      apply = () => {
        const d = theme === "dark" || (theme === "system" && mq.matches);
        document.documentElement.dataset.theme = d ? "dark" : "light";
        setDark(d);
      };
    apply();
    mq.addEventListener("change", apply);
    return () => mq.removeEventListener("change", apply);
  }, [theme]);
  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(""), 6000);
    return () => clearTimeout(timer);
  }, [toast]);
  useEffect(() => {
    if (!modal) return;
    const previous = document.activeElement;
    const dialog = document.querySelector("[role=dialog]");
    const focusable = () =>
      [...dialog.querySelectorAll("button,input,textarea,select")].filter(
        (x) => !x.disabled && x.offsetParent !== null,
      );
    dialog.querySelector("input,select,textarea")?.focus();
    const key = (e) => {
      if (e.key === "Escape" && !busy) {
        e.preventDefault();
        setModal(null);
      }
      if (e.key === "Tab") {
        const nodes = focusable(),
          first = nodes[0],
          last = nodes.at(-1);
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last?.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first?.focus();
        }
      }
    };
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("keydown", key);
      previous?.focus();
    };
  }, [modal, busy]);
  useEffect(() => {
    const mobile = matchMedia("(max-width: 1000px)");
    const resize = (event) => {
      if (!event.matches) setMenu(false);
    };
    const key = (e) => {
      if (e.key === "Escape") setMenu(false);
    };
    mobile.addEventListener("change", resize);
    document.addEventListener("keydown", key);
    return () => {
      mobile.removeEventListener("change", resize);
      document.removeEventListener("keydown", key);
    };
  }, []);
  useEffect(() => {
    if (!modal && !(menu && matchMedia("(max-width: 1000px)").matches)) return;
    const oldOverflow = document.body.style.overflow,
      oldPadding = document.body.style.paddingRight;
    const gap = innerWidth - document.documentElement.clientWidth;
    document.body.style.overflow = "hidden";
    if (gap > 0) document.body.style.paddingRight = `${gap}px`;
    return () => {
      document.body.style.overflow = oldOverflow;
      document.body.style.paddingRight = oldPadding;
    };
  }, [modal, menu]);
  async function api(path, body) {
    const r = await fetch("/api/" + path, {
      method: body === undefined ? "GET" : "POST",
      headers:
        body === undefined
          ? {}
          : { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const v = await r.json();
    if (!r.ok) {
      if (r.status === 401 && !restoreInFlight.current) setAuth("no");
      throw Error(v.error || `HTTP ${r.status}`);
    }
    return v;
  }
  useEffect(() => {
    if (!demo)
      api("session")
        .then((v) => {
          setCsrf(v.csrf);
          setAuth("yes");
        })
        .catch(() => setAuth("no"));
  }, []);
  async function refresh() {
    if (!demo) {
      const v = await api(
        `snapshot?period=${[1, 10].includes(page) ? period : "day"}&date=${[1, 10].includes(page) ? date : new Date().toLocaleDateString("en-CA")}&window=${speedWindow}&group=${page === 10 ? "downloaders" : "sites"}&view=${page === 0 ? "active" : page === 8 ? "all" : page === 11 ? "logs" : "none"}&downloadOffset=${downloadPage * 50}&offset=${torrentPage * 50}&search=${encodeURIComponent(page === 8 ? search : "")}`,
      );
      setData(v);
    }
  }
  useEffect(() => {
    if (auth !== "yes" || demo) return;
    let alive = true,
      inFlight = false;
    const update = async () => {
      if (inFlight || document.hidden) return;
      inFlight = true;
      try {
        const v = await api(
          `snapshot?period=${[1, 10].includes(page) ? period : "day"}&date=${[1, 10].includes(page) ? date : new Date().toLocaleDateString("en-CA")}&window=${speedWindow}&group=${page === 10 ? "downloaders" : "sites"}&view=${page === 0 ? "active" : page === 8 ? "all" : page === 11 ? "logs" : "none"}&downloadOffset=${downloadPage * 50}&offset=${torrentPage * 50}&search=${encodeURIComponent(page === 8 ? search : "")}`,
        );
        if (alive) {
          setData(v);
          setError("");
        }
      } catch (e) {
        if (alive) setError(e.message);
      } finally {
        inFlight = false;
      }
    };
    update();
    const timer = setInterval(update, 5000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [
    auth,
    period,
    date,
    page,
    torrentPage,
    downloadPage,
    search,
    speedWindow,
  ]);
  const totals = useMemo(() => {
    const v = { up: 0, down: 0, sites: {}, uncertain: false };
    for (const b of data?.buckets || []) {
      v.up += b.up;
      v.down += b.down;
      v.uncertain ||= b.uncertain;
      v.sites[b.site] ||= { up: 0, down: 0 };
      v.sites[b.site].up += b.up;
      v.sites[b.site].down += b.down;
    }
    return page === 0 && data?.lifetime ? { ...v, ...data.lifetime } : v;
  }, [data, page]);
  const buckets = useMemo(() => {
    const m = {};
    for (const b of data?.buckets || []) {
      m[b.time] ||= { time: b.time, up: 0, down: 0 };
      m[b.time].up += b.up;
      m[b.time].down += b.down;
    }
    return Object.values(m).sort((a, b) => a.time - b.time);
  }, [data]);
  const speed = (data?.clients || [])
      .filter((x) => x.online)
      .reduce((s, x) => s + x.up, 0),
    siteName = (k) =>
      c?.sites.find((s) => s.id === k)?.name ||
      t("历史归属", "Historical attribution"),
    names = [
      t("流量总览", "Overview"),
      t("流量统计", "Statistics"),
      t("带宽出口", "Bandwidth outlets"),
      t("下载管理", "Downloaders"),
      t("系统设置", "Settings"),
      t("回调通知", "Webhook"),
      t("备份恢复", "Backup & restore"),
      t("关于程序", "About"),
      t("站点策略", "Site policies"),
      t("站点识别", "Site recognition"),
      t("下载器统计", "Downloader statistics"),
      t("运行记录", "Activity log"),
    ],
    nav = [
      House,
      ChartBar,
      Stack,
      DownloadSimple,
      GearSix,
      Broadcast,
      FloppyDisk,
      Info,
    ];
  const group = [1, 10].includes(page)
    ? 1
    : [3, 2, 8, 11].includes(page)
      ? 3
      : [4, 9, 5, 6].includes(page)
        ? 4
        : page;
  const tabs =
    group === 1
      ? [
          [1, t("站点统计", "By site")],
          [10, t("下载器统计", "By downloader")],
        ]
      : group === 3
        ? [
            [3, t("下载器", "Downloaders")],
            [2, t("带宽出口", "Bandwidth outlets")],
            [8, t("站点策略", "Site policies")],
            [11, t("运行记录", "Activity log")],
          ]
        : group === 4
          ? [
              [4, t("常规设置", "General")],
              [9, t("站点识别", "Site recognition")],
              [5, t("回调通知", "Webhook")],
              [6, t("备份恢复", "Backup & restore")],
            ]
          : [];
  function open(kind, v = {}) {
    if (kind === "pool") {
      const schedule = v.schedule || {};
      const nightBudget =
        schedule.nightBudget || rateBytes(v.budget) || 6250000;
      v = {
        ...v,
        schedule: {
          enabled: !!schedule.enabled,
          nightBudget,
          start: schedule.start || "23:00",
          end: schedule.end || "08:00",
          days: Array.isArray(schedule.days)
            ? schedule.days
            : [0, 1, 2, 3, 4, 5, 6],
        },
        nightKB: rateInput(nightBudget),
      };
    }
    setModal(kind);
    setError("");
    setForm(structuredClone(v));
  }
  function editSite(s) {
    open("site", {
      ...s,
      domainText: s.domains.join("\n"),
      minimum: rateInput(s.minimum || 50000),
      maximum: rateInput(s.maximum),
      afterTarget: rateInput(s.afterTarget),
      dayTarget: s.dayTarget / GB,
      monthTarget: s.monthTarget / GB,
    });
  }
  function newSite() {
    editSite({
      id: id(),
      name: "",
      domains: [],
      regex: "",
      weight: 10,
      minimum: 50000,
      maximum: 0,
      dayTarget: 0,
      monthTarget: 0,
      afterTarget: 50000,
    });
  }
  function newClient() {
    open("client", {
      id: id(),
      name: "",
      kind: "qb",
      url: "",
      user: "",
      password: "",
      pool:
        c.pools.find((p) => p.id === "default-outlet")?.id ||
        c.pools[0]?.id ||
        "",
      autoManage: false,
      enabled: true,
    });
  }
  async function catalogAction(body) {
    if (body.auto !== undefined && !demo)
      setData((d) => ({ ...d, catalog: { ...d.catalog, auto: body.auto } }));
    setBusy(true);
    setError("");
    try {
      if (!demo) {
        const catalog = await api("site-catalog", body);
        setData((d) => ({ ...d, catalog }));
      } else
        setToast(t("演示模式，不修改站点库", "Demo mode; catalog unchanged"));
    } catch (e) {
      setError(e.message);
      if (!demo) await refresh();
    } finally {
      setBusy(false);
    }
  }
  async function saveConfig(next) {
    if (demo) {
      setToast(
        t("演示模式，不会修改下载器", "Demo mode; no downloader changes"),
      );
      setModal(null);
      return;
    }
    setBusy(true);
    try {
      const v = await api("config", next);
      setData((d) => ({ ...d, config: v }));
      setModal(null);
      setToast(
        t(
          "设置已保存，等待采样同步",
          "Saved; synchronization runs on the next sample",
        ),
      );
      await refresh();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  async function submit(ev) {
    ev.preventDefault();
    if (modal === "password" || modal === "match") {
      setBusy(true);
      try {
        if (modal === "password") {
          if (!demo) await api("password", form);
          setAuth("no");
          setModal(null);
          setToast(
            t(
              "密码已修改，所有旧登录已失效",
              "Password changed; all old sessions revoked",
            ),
          );
        } else {
          const result = demo
            ? { host: new URL(form.url).hostname, site: "s0" }
            : await api("match", { url: form.url });
          setForm((f) => ({ ...f, result }));
        }
      } catch (e) {
        setError(e.message);
      } finally {
        setBusy(false);
      }
      return;
    }
    const next = structuredClone(c);
    if (modal === "torrent")
      next.overrides[form.key] = {
        site: form.site || "",
        weight: Number(form.weight || 1),
        manage: !!form.manage,
      };
    else if (modal === "notify")
      next.notify = { ...form, headersStored: undefined };
    else if (modal === "preferences") {
      next.timezone = form.timezone;
      next.pollSeconds = Number(form.pollSeconds);
      next.idleSeconds = Number(form.idleSeconds);
      next.uploadConfirmSeconds = Number(form.uploadConfirmSeconds);
    } else {
      const field = { client: "clients", site: "sites", pool: "pools" }[modal],
        v = { ...form };
      if (modal === "site") {
        v.domains = v.domainText.split(/[\n,\s]+/).filter(Boolean);
        delete v.domainText;
        const previous = c.sites.find((site) => site.id === v.id);
        if (
          !previous ||
          JSON.stringify(previous.domains) !== JSON.stringify(v.domains) ||
          previous.regex !== v.regex
        )
          v.automatic = false;
        for (const k of ["minimum", "maximum", "afterTarget"])
          v[k] = rateBytes(v[k]);
        for (const k of ["dayTarget", "monthTarget"])
          v[k] = Math.round(Number(v[k] || 0) * GB);
        v.weight = Number(v.weight);
      }
      if (modal === "pool") {
        v.enabled = true;
        v.budget = rateBytes(v.budget);
        v.schedule = {
          ...v.schedule,
          nightBudget: rateBytes(v.nightKB),
        };
        delete v.nightKB;
        if (!next.pools.length)
          next.clients = next.clients.map((cl) =>
            cl.pool ? cl : { ...cl, pool: v.id },
          );
      }
      const i = next[field].findIndex((x) => x.id === v.id);
      if (i < 0) next[field].push(v);
      else next[field][i] = v;
    }
    await saveConfig(next);
  }
  async function remove() {
    if (
      !confirm(
        t(
          "删除此配置？将先恢复系统限速。",
          "Delete configuration? System limits will be restored first.",
        ),
      )
    )
      return;
    const next = structuredClone(c),
      field = { client: "clients", site: "sites", pool: "pools" }[modal];
    next[field] = next[field].filter((v) => v.id !== form.id);
    if (modal === "site")
      for (const o of Object.values(next.overrides))
        if (o.site === form.id) o.site = "";
    if (modal === "pool")
      for (const cl of next.clients) if (cl.pool === form.id) cl.pool = "";
    await saveConfig(next);
  }
  async function testWebhook() {
    setBusy(true);
    try {
      if (!demo) await api("test-webhook", form);
      setToast(
        t("测试通知已发送，配置未保存", "Test sent; settings were not saved"),
      );
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  async function restoreBackup() {
    if (!backupFile) return;
    if (
      !confirm(
        t(
          "恢复将覆盖当前数据，服务端会保留恢复前备份。请先停止所有下载器策略并等待限速清空；恢复后重新登录，自动策略保持暂停。继续？",
          "Restore replaces current data and preserves a rollback backup. Stop downloader strategies and wait for limit clearance first. Sign in again after restore; strategies remain paused. Continue?",
        ),
      )
    )
      return;
    restoreInFlight.current = true;
    setBusy(true);
    try {
      const r = await fetch("/api/backup", {
        method: "POST",
        headers: {
          "X-CSRF-Token": csrf,
          "X-Confirm-Restore": "replace",
          "Content-Type": "application/zip",
        },
        body: backupFile,
      });
      const v = await r.json();
      if (!r.ok) throw Error(v.error || "Restore failed");
      location.replace("/");
    } catch (e) {
      restoreInFlight.current = false;
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  async function testClient() {
    setBusy(true);
    try {
      const v = demo ? { torrents: 3 } : await api("test-client", form);
      if (form.kind === "tr" && v.endpoint) {
        setForm((f) => ({ ...f, url: v.endpoint }));
      }
      setToast(
        t(
          `连接成功，发现 ${v.torrents} 个种子`,
          `Connected: ${v.torrents} torrents`,
        ),
      );
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  const controls = (
    <>
      <div className="group">
        {[
          ["light", Sun, t("浅色", "Light")],
          ["dark", Moon, t("深色", "Dark")],
          ["system", Desktop, t("跟随系统", "System")],
        ].map(([k, I, label]) => (
          <button
            key={k}
            aria-label={label}
            title={label}
            className={theme === k ? "active" : ""}
            onClick={() => setTheme(k)}
          >
            <I size={17} />
          </button>
        ))}
      </div>
      <button
        className="icon language-toggle"
        aria-label={t("切换为英文", "Switch to Chinese")}
        title={t("切换为英文", "Switch to Chinese")}
        onClick={() => setLang(lang === "zh" ? "en" : "zh")}
      >
        {lang === "zh" ? "EN" : "中"}
      </button>
    </>
  );
  const periods = (
    <div className="group">
      {[
        ["day", "日", "Day"],
        ["week", "周", "Week"],
        ["month", "月", "Month"],
        ["year", "年", "Year"],
      ].map(([k, zh, en]) => (
        <button
          key={k}
          className={period === k ? "active" : ""}
          onClick={() => setPeriod(k)}
        >
          {t(zh, en)}
        </button>
      ))}
    </div>
  );
  const guard = (
    <span className={"badge " + (!data?.guardian ? "warn" : "")}>
      <ShieldCheck size={15} />
      {data?.guardian
        ? t("恢复保护在线", "Recovery online")
        : t("恢复保护未连接", "Recovery offline")}
    </span>
  );
  const siteTable = (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{t("站点", "Site")}</th>
            <th>{t("上传", "Upload")}</th>
            <th>{t("下载", "Download")}</th>
            <th className="hide-phone">{t("上传速度", "Upload rate")}</th>
            <th>{t("权重", "Weight")}</th>
          </tr>
        </thead>
        <tbody>
          {[
            ...(c?.sites || []),
            ...(totals.sites[""]
              ? [
                  {
                    id: "",
                    name: t("历史归属", "Historical attribution"),
                    weight: "—",
                    domains: [],
                  },
                ]
              : []),
          ].map((s) => (
            <tr key={s.id}>
              <td className="name">
                <span
                  className="dot"
                  style={{ background: siteColor(s.id, c.sites) }}
                />
                {s.name}
              </td>
              <td>{bytes(totals.sites[s.id]?.up)}</td>
              <td>{bytes(totals.sites[s.id]?.down)}</td>
              <td className="hide-phone">
                {bytes(
                  data.siteRates?.[s.id] ??
                    data.torrents
                      .filter((x) => x.site === s.id)
                      .reduce((a, x) => a + x.up, 0),
                  true,
                )}
              </td>
              <td>
                <span className="badge">{s.weight}</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {!c?.sites.length && !Object.keys(totals.sites).length && (
        <div className="empty">
          <Globe size={30} />
          {t(
            "连接下载器后自动发现 Tracker，点击站点修改名称",
            "Trackers appear automatically; rename sites as needed.",
          )}
          <br />
          <button onClick={newSite}>
            <Plus />
            {t("添加站点", "Add site")}
          </button>
        </div>
      )}
    </div>
  );
  const clients = (
    <>
      {c?.clients.map((cl) => {
        const s = data.clients.find((v) => v.id === cl.id);
        return (
          <div className="client-row" key={cl.id}>
            <div className={"client-symbol " + cl.kind}>
              {cl.kind === "qb" ? "qB" : "TR"}
            </div>
            <div className="grow">
              <h3>{cl.name}</h3>
              <small>
                {t("带宽出口：", "Outlet: ")}
                {c.pools.find((p) => p.id === cl.pool)?.name ||
                  t("默认出口", "Default outlet")}{" "}
                · {bytes(c.pools.find((p) => p.id === cl.pool)?.budget, true)}
              </small>
              <small>
                <span
                  className={
                    "dot " + (!cl.enabled || !s?.online ? "offline" : "")
                  }
                />
                {!cl.enabled
                  ? t("未启用", "Disabled")
                  : s?.statisticsError
                    ? t("已连接，统计写入失败", "Online · statistics write failed")
                    : s?.online
                      ? t("运行中", "Online")
                      : s?.failures >= 3
                        ? t("连续采样失败", "Sampling repeatedly failed")
                        : s?.failures
                          ? t("采样失败，等待重试", "Sampling failed · retrying")
                          : t("等待首次采样", "Waiting for first sample")}
              </small>
              {cl.enabled && s?.error && <small>{s.error}</small>}
              {cl.enabled && !s?.online && s?.lastSuccess > 0 && (
                <small>{t("最近成功采样：", "Last successful sample: ")}{new Date(s.lastSuccess * 1000).toLocaleString()}</small>
              )}
              {cl.clearPending && (
                <small role="status">
                  {t(
                    "限速清空待重试，完成后才能重新启动策略",
                    "Limit clearance pending; wait before restarting allocation",
                  )}
                </small>
              )}
            </div>
            <div className="speeds">
              <span className="up">
                ↑ {bytes(cl.enabled ? s?.up : 0, true)}
              </span>
              <span className="down">
                ↓ {bytes(cl.enabled ? s?.down : 0, true)}
              </span>
            </div>
            <div className="client-actions">
              <button
                disabled={busy || cl.restorePending || cl.clearPending}
                onClick={async () => {
                  if (
                    cl.enabled &&
                    !confirm(
                      t(
                        "停用后停止采集和分配，并恢复系统施加的限速？",
                        "Disable monitoring and allocation, restoring system-owned limits?",
                      ),
                    )
                  )
                    return;
                  setBusy(true);
                  try {
                    if (!demo)
                      await api("client-enabled", {
                        id: cl.id,
                        enabled: !cl.enabled,
                      });
                    else
                      setData((d) => ({
                        ...d,
                        config: {
                          ...d.config,
                          clients: d.config.clients.map((x) =>
                            x.id === cl.id ? { ...x, enabled: !cl.enabled } : x,
                          ),
                        },
                      }));
                    await refresh();
                  } catch (e) {
                    setError(e.message);
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                {cl.enabled
                  ? t("停用下载器", "Disable downloader")
                  : t("启用下载器", "Enable downloader")}
              </button>
              {cl.restorePending && (
                <small role="status">
                  {t("系统限速恢复待重试", "System limit recovery pending")}
                </small>
              )}
              <button
                disabled={
                  busy || !cl.enabled || cl.restorePending || cl.clearPending
                }
                className={
                  cl.strategyEnabled === false || c.policyEnabled === false
                    ? ""
                    : "primary"
                }
                onClick={async () => {
                  const enabled =
                    cl.strategyEnabled === false || c.policyEnabled === false;
                  if (
                    !enabled &&
                    !confirm(
                      t(
                        "停止该下载器自动策略并清空其种子级上传限速？",
                        "Stop this downloader strategy and clear its torrent upload limits?",
                      ),
                    )
                  )
                    return;
                  setBusy(true);
                  try {
                    if (!demo)
                      await api("client-policy", { id: cl.id, enabled });
                    else
                      setData((d) => ({
                        ...d,
                        config: {
                          ...d.config,
                          clients: d.config.clients.map((x) =>
                            x.id === cl.id
                              ? { ...x, strategyEnabled: enabled }
                              : x,
                          ),
                        },
                      }));
                    await refresh();
                  } catch (e) {
                    setError(e.message);
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                {cl.strategyEnabled === false || c.policyEnabled === false
                  ? t("启动自动策略", "Start strategy")
                  : t("停止自动策略", "Stop strategy")}
              </button>
              {page === 3 && (
                <button
                  className="icon"
                  aria-label={t("编辑", "Edit")}
                  onClick={() => open("client", cl)}
                >
                  <GearSix />
                </button>
              )}
            </div>
          </div>
        );
      })}
      {!c?.clients.length && (
        <div className="empty">
          <DownloadSimple size={30} />
          {t("连接你的第一个下载器", "Connect your first downloader")}
          <br />
          <button onClick={newClient}>
            <Plus />
            {t("添加下载器", "Add downloader")}
          </button>
        </div>
      )}
    </>
  );
  const stateLabel = (v) =>
    ({
      allocated: t("分配中", "Allocated"),
      released: t("已释放", "Released"),
      monitoring: t("监控中", "Monitoring"),
      "release pending": t("恢复待重试", "Recovery pending"),
    })[v] || v;
  const serverPaged = data?.torrentTotal !== undefined;
  const visibleTorrents = (data?.torrents || []).filter(
    (x) =>
      (serverPaged ||
        page !== 8 ||
        !search ||
        x.name.toLowerCase().includes(search.toLowerCase()) ||
        (x.hosts || []).some((h) =>
          h.toLowerCase().includes(search.toLowerCase()),
        )) &&
      (serverPaged || page !== 0 || x.connections > 0 || x.up > 0),
  );
  const downloading =
    data?.downloading ||
    (demo
      ? data.torrents.filter((row) => row.downloading || row.down > 0)
      : []);
  const downloadTotal = data?.downloadingTotal ?? downloading.length;
  const downloadCurrent = Math.floor(
    (data?.downloadOffset ?? downloadPage * 50) / 50,
  );
  const torrentCount = data?.torrentTotal ?? visibleTorrents.length;
  const lastTorrentPage = Math.max(0, Math.ceil(torrentCount / 50) - 1);
  const currentTorrentPage = Math.min(torrentPage, lastTorrentPage);
  const torrents = (
    <div>
      <p className="hint">
        {t(
          "拖动表头分隔线调整列宽；窄屏可左右滑动。列宽仅保存在此设备。",
          "Drag header separators to resize columns; swipe horizontally on narrow screens. Widths stay on this device.",
        )}
      </p>
      <div className="table-wrap">
        <table
          className="torrent-table"
          style={{ minWidth: columnWidths.reduce((n, v) => n + v, 0) }}
        >
          <colgroup>
            {columnWidths.map((width, i) => (
              <col key={i} style={{ width }} />
            ))}
          </colgroup>
          <thead>
            <tr>
              {[
                ["种子名称", "Torrent name"],
                ["当前上传", "Upload rate"],
                ["当前限速", "Limit"],
                ["状态", "State"],
                ["下载器", "Downloader"],
                ["站点", "Site"],
                ["上传连接", "Connections"],
                ["Tracker", "Tracker"],
              ].map(([zh, en], i) => (
                <th key={i} aria-label={t(zh, en)}>
                  {t(zh, en)}
                  <span
                    className="column-handle"
                    role="separator"
                    aria-orientation="vertical"
                    aria-label={t("调整列宽：", "Resize column: ") + t(zh, en)}
                    aria-valuemin={i === 0 ? 180 : 70}
                    aria-valuemax={1600}
                    aria-valuenow={columnWidths[i]}
                    tabIndex={0}
                    onPointerDown={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      columnDrag.current = {
                        index: i,
                        x: e.clientX,
                        width: columnWidths[i],
                      };
                      e.currentTarget.setPointerCapture(e.pointerId);
                    }}
                    onPointerMove={(e) => {
                      const d = columnDrag.current;
                      if (!d || d.index !== i) return;
                      const width = Math.min(
                        1600,
                        Math.max(i === 0 ? 180 : 70, d.width + e.clientX - d.x),
                      );
                      setColumnWidths((v) =>
                        v.map((n, j) => (j === i ? width : n)),
                      );
                    }}
                    onPointerUp={() => {
                      columnDrag.current = null;
                    }}
                    onPointerCancel={() => {
                      columnDrag.current = null;
                    }}
                    onKeyDown={(e) => {
                      if (e.key !== "ArrowLeft" && e.key !== "ArrowRight")
                        return;
                      e.preventDefault();
                      const delta = e.key === "ArrowLeft" ? -20 : 20;
                      setColumnWidths((v) =>
                        v.map((n, j) =>
                          j === i
                            ? Math.min(
                                1600,
                                Math.max(i === 0 ? 180 : 70, n + delta),
                              )
                            : n,
                        ),
                      );
                    }}
                  />
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {(serverPaged
              ? visibleTorrents
              : visibleTorrents.slice(
                  currentTorrentPage * 50,
                  (currentTorrentPage + 1) * 50,
                )
            ).map((x) => (
              <tr
                key={x.key}
                className="action-row"
                onClick={() =>
                  open("torrent", {
                    key: x.key,
                    name: x.name,
                    site: x.site,
                    weight: c.overrides[x.key]?.weight || 1,
                    manage:
                      c.overrides[x.key]?.manage ??
                      !!c.clients.find((cl) => cl.id === x.client)?.autoManage,
                  })
                }
              >
                <td className="name" title={x.name}>
                  <span className="torrent-name">{x.name}</span>
                </td>
                <td className="up">{bytes(x.up, true)}</td>
                <td>
                  {x.limit ? bytes(x.limit, true) : t("不限速", "Unlimited")}
                </td>
                <td>
                  <span
                    className={"badge state " + (!x.managed ? "neutral" : "")}
                  >
                    {stateLabel(x.state)}
                  </span>
                </td>
                <td
                  title={
                    c.clients.find((cl) => cl.id === x.client)?.name ||
                    x.client ||
                    ""
                  }
                >
                  {c.clients.find((cl) => cl.id === x.client)?.name ||
                    x.client ||
                    "—"}
                </td>
                <td title={siteName(x.site)}>{siteName(x.site)}</td>
                <td>{x.connections}</td>
                <td title={(x.hosts || []).join(" · ")}>
                  {(x.hosts || []).join(" · ") || t("暂不可用", "Unavailable")}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {torrentCount > 50 && (
          <div className="toolbar">
            <button
              disabled={currentTorrentPage === 0}
              onClick={() => setTorrentPage(currentTorrentPage - 1)}
            >
              {t("上一页", "Previous")}
            </button>
            <span>
              {currentTorrentPage + 1} / {lastTorrentPage + 1} · {torrentCount}
            </span>
            <button
              disabled={currentTorrentPage === lastTorrentPage}
              onClick={() => setTorrentPage(currentTorrentPage + 1)}
            >
              {t("下一页", "Next")}
            </button>
          </div>
        )}
        {!visibleTorrents.length && (
          <div className="empty">
            {t(
              page === 0
                ? "暂无符合分配条件的种子"
                : "没有匹配的种子，连接下载器后自动采集",
              page === 0
                ? "No torrents qualify for allocation"
                : "No matching torrents. Connect a downloader to begin.",
            )}
          </div>
        )}
      </div>
    </div>
  );
  const field = (key, zh, en, type = "text", extra = {}) => (
    <label key={key}>
      {t(zh, en)}
      <input
        type={type}
        {...extra}
        value={form[key] ?? ""}
        onChange={(e) => setForm({ ...form, [key]: e.target.value })}
      />
    </label>
  );
  const toggle = (key, zh, en) => (
    <label className="checkbox">
      <input
        type="checkbox"
        checked={!!form[key]}
        onChange={(e) => setForm({ ...form, [key]: e.target.checked })}
      />
      {t(zh, en)}
    </label>
  );
  const select = (key, zh, en, options) => (
    <label>
      {t(zh, en)}
      <select
        value={form[key] || ""}
        onChange={(e) => setForm({ ...form, [key]: e.target.value })}
      >
        {options.map(([v, label]) => (
          <option value={v} key={v}>
            {label}
          </option>
        ))}
      </select>
    </label>
  );
  if (auth === "loading")
    return (
      <div className="loading">
        Torrent Flow · {t("正在连接…", "Connecting…")}
      </div>
    );
  if (auth === "no")
    return (
      <div className="login">
        <div className="login-box">
          <div className="brand">
            <WaveSine size={35} />
            <div>
              {t("流量台", "Torrent Flow")}
              <small>{t("下载器流量统计", "Torrent traffic monitor")}</small>
            </div>
          </div>
          <h1>{t("欢迎回来", "Welcome back")}</h1>
          <p className="muted">
            {t(
              "登录以查看流量与管理下载器",
              "Sign in to view traffic and manage downloaders",
            )}
          </p>
          <form
            className="fields"
            onSubmit={async (ev) => {
              ev.preventDefault();
              setBusy(true);
              try {
                const v = await api("login", {
                  user: form.user || "admin",
                  password: form.password || "",
                });
                setCsrf(v.csrf);
                setAuth("yes");
                setForm({});
                setError("");
              } catch (e) {
                setError(e.message);
              } finally {
                setBusy(false);
              }
            }}
          >
            {field("user", "账号", "Username", "text", {
              autoComplete: "username",
              placeholder: "admin",
            })}
            {field("password", "密码", "Password", "password", {
              autoComplete: "current-password",
              required: true,
            })}
            {error && (
              <p className="error" role="alert">
                {error}
              </p>
            )}
            <button className="primary" disabled={busy}>
              {t("登录", "Sign in")}
              <ArrowRight />
            </button>
          </form>
        </div>
      </div>
    );
  if (!data)
    return (
      <div className="loading">
        {error || t("正在载入…", "Loading…")}{" "}
        <button onClick={() => refresh().catch((e) => setError(e.message))}>
          <ArrowClockwise />
        </button>
      </div>
    );
  return (
    <div>
      {menu && (
        <div
          className="backdrop-nav"
          aria-hidden="true"
          onClick={() => setMenu(false)}
        />
      )}
      <aside className={"sidebar " + (menu ? "open" : "")}>
        <div className="brand">
          <WaveSine size={34} />
          <div>
            {t("流量台", "Torrent Flow")}
            <small>Torrent Flow</small>
          </div>
        </div>
        <nav className="nav" id="main-navigation">
          {[0, 1, 3, 4, 7].map((i) => {
            const name = names[i];
            const I = nav[i];
            return (
              <button
                key={i}
                className={group === i ? "active" : ""}
                onClick={() => {
                  setPage(i);
                  setMenu(false);
                  setSearch("");
                }}
              >
                <I />
                {name}
              </button>
            );
          })}
        </nav>
        <div className="sidebar-footer">
          {t("累计计数 · 精确增量", "Cumulative counters · measured deltas")}
          <br />v{data.version} · MIT License
        </div>
      </aside>
      <main className="main">
        <header className="topbar">
          <button
            className="icon menu-toggle"
            aria-label={t("打开菜单", "Open navigation")}
            aria-expanded={menu}
            aria-controls="main-navigation"
            onClick={() => setMenu(!menu)}
          >
            <List size={22} />
          </button>
          <div className="page-title">
            <h1>{names[group]}</h1>
          </div>
          <div className="controls">
            {controls}
            <button
              className="icon"
              aria-label={t("退出登录", "Sign out")}
              onClick={async () => {
                if (!demo) await api("logout", {});
                setAuth("no");
              }}
            >
              <SignOut size={18} />
            </button>
          </div>
        </header>
        {demo && (
          <div className="notice demo">
            {t(
              "界面演示 · 示例数据，不连接真实下载器",
              "UI demo · sample data, no live downloader connection",
            )}
          </div>
        )}
        {!!tabs.length && (
          <div
            className="section-tabs"
            role="tablist"
            aria-label={t("页面分类", "Page sections")}
          >
            {tabs.map(([key, label]) => (
              <button
                key={key}
                role="tab"
                aria-selected={page === key}
                onClick={() => {
                  setPage(key);
                  setSearch("");
                }}
              >
                {label}
              </button>
            ))}
          </div>
        )}
        {error && !modal && (
          <div className="notice error" role="alert">
            {error}
          </div>
        )}
        {c.clearPending && (
          <div className="notice" role="status">
            {t(
              "策略已停止，部分下载器的限速清空尚未完成，系统会重试。",
              "Strategy stopped. Some limit clearance tasks are pending and will retry.",
            )}
          </div>
        )}
        {data.warnings?.map((v) => (
          <div className="notice" key={v}>
            {v}
          </div>
        ))}
        {[0, 1, 10].includes(page) && (
          <>
            <div
              className={
                "toolbar filters " + (page === 0 ? "overview-filters" : "")
              }
            >
              {[1, 10].includes(page) && periods}
              {[1, 10].includes(page) && period === "week" && (
                <div className="period-selector">
                  <select
                    aria-label={t("周所属年份", "Week year")}
                    value={isoWeekValue(date).slice(0, 4)}
                    onChange={(e) =>
                      setDate(
                        weekStart(
                          e.target.value +
                            "-W" +
                            String(
                              Math.min(
                                Number(isoWeekValue(date).slice(6)),
                                Number(
                                  isoWeekValue(e.target.value + "-12-28").slice(
                                    6,
                                  ),
                                ),
                              ),
                            ).padStart(2, "0"),
                        ),
                      )
                    }
                  >
                    {[
                      ...new Set([
                        new Date().getFullYear(),
                        new Date().getFullYear() - 1,
                        Number(isoWeekValue(date).slice(0, 4)),
                      ]),
                    ]
                      .sort((a, b) => b - a)
                      .map((y) => (
                        <option key={y} value={y}>
                          {y}
                        </option>
                      ))}
                  </select>
                  <select
                    aria-label={t("选择统计周", "Choose report week")}
                    value={Number(isoWeekValue(date).slice(6))}
                    onChange={(e) =>
                      setDate(
                        weekStart(
                          isoWeekValue(date).slice(0, 4) +
                            "-W" +
                            String(e.target.value).padStart(2, "0"),
                        ),
                      )
                    }
                  >
                    {Array.from(
                      {
                        length: Number(
                          isoWeekValue(
                            isoWeekValue(date).slice(0, 4) + "-12-28",
                          ).slice(6),
                        ),
                      },
                      (_, i) => {
                        const n = i + 1,
                          start = weekStart(
                            isoWeekValue(date).slice(0, 4) +
                              "-W" +
                              String(n).padStart(2, "0"),
                          ),
                          end = new Date(
                            Date.parse(start + "T00:00:00Z") + 6 * 86400000,
                          )
                            .toISOString()
                            .slice(0, 10);
                        return (
                          <option
                            key={n}
                            value={n}
                            disabled={
                              Date.parse(start + "T00:00:00Z") >
                              Date.now() + 86400000
                            }
                          >
                            {t("第", "Week ")}
                            {n}
                            {t("周 · ", " · ")}
                            {start.slice(5)} — {end.slice(5)}
                          </option>
                        );
                      },
                    )}
                  </select>
                </div>
              )}
              {[1, 10].includes(page) && period === "month" && (
                <div className="period-selector">
                  <select
                    aria-label={t("月份所属年份", "Month year")}
                    value={date.slice(0, 4)}
                    onChange={(e) =>
                      setDate(e.target.value + date.slice(4, 7) + "-01")
                    }
                  >
                    {[
                      new Date().getFullYear(),
                      new Date().getFullYear() - 1,
                    ].map((y) => (
                      <option key={y} value={y}>
                        {y}
                      </option>
                    ))}
                  </select>
                  <select
                    aria-label={t("选择统计月份", "Choose report month")}
                    value={Number(date.slice(5, 7))}
                    onChange={(e) =>
                      setDate(
                        date.slice(0, 4) +
                          "-" +
                          String(e.target.value).padStart(2, "0") +
                          "-01",
                      )
                    }
                  >
                    {Array.from({ length: 12 }, (_, i) => (
                      <option
                        key={i}
                        value={i + 1}
                        disabled={
                          Number(date.slice(0, 4)) ===
                            new Date().getFullYear() &&
                          i > new Date().getMonth()
                        }
                      >
                        {i + 1}
                        {t("月", "")}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              {[1, 10].includes(page) && period === "year" && (
                <select
                  aria-label={t("选择统计年份", "Choose report year")}
                  value={date.slice(0, 4)}
                  onChange={(e) => setDate(e.target.value + "-01-01")}
                >
                  {[new Date().getFullYear(), new Date().getFullYear() - 1].map(
                    (y) => (
                      <option key={y} value={y}>
                        {y}
                      </option>
                    ),
                  )}
                </select>
              )}
              {[1, 10].includes(page) && period === "day" && (
                <input
                  className="date"
                  type={
                    period === "year"
                      ? "number"
                      : period === "month"
                        ? "month"
                        : period === "week"
                          ? "week"
                          : "date"
                  }
                  aria-label={t("选择统计周期", "Choose report period")}
                  value={
                    period === "year"
                      ? date.slice(0, 4)
                      : period === "month"
                        ? date.slice(0, 7)
                        : period === "week"
                          ? isoWeekValue(date)
                          : date
                  }
                  min={
                    period === "year" ? new Date().getFullYear() - 1 : undefined
                  }
                  max={period === "year" ? new Date().getFullYear() : undefined}
                  onChange={(e) => {
                    const v = e.target.value;
                    if (!v) return;
                    const d =
                      period === "year"
                        ? v.length === 4
                          ? v + "-01-01"
                          : ""
                        : period === "month"
                          ? v + "-01"
                          : period === "week"
                            ? weekStart(v)
                            : v;
                    if (d) setDate(d);
                  }}
                />
              )}
              {[1, 10].includes(page) && (
                <small>
                  {new Date(data.start * 1000).toLocaleDateString()} —{" "}
                  {new Date((data.end - 1) * 1000).toLocaleDateString()} ·{" "}
                  {t(
                    "最多保留一年，较早日期可能不完整",
                    "One-year retention; earlier periods may be partial",
                  )}
                </small>
              )}
              <span className="grow" />
            </div>
            <div className="stats">
              {[
                [
                  ArrowUp,
                  page === 0 ? t("累计上传", "Total upload") : t("区间上传", "Period upload"),
                  bytes(totals.up),
                  "up",
                ],
                [
                  ArrowDown,
                  page === 0 ? t("累计下载", "Total download") : t("区间下载", "Period download"),
                  bytes(totals.down),
                  "down",
                ],
                [
                  Broadcast,
                  t("当前上传", "Current upload"),
                  bytes(speed, true),
                  "up",
                ],
              ].map(([I, label, value, cls]) => (
                <div className={"stat " + cls} key={label}>
                  <div className="stat-icon">
                    <I size={23} />
                  </div>
                  <div>
                    <div className="stat-label">{label}</div>
                    <strong>{value}</strong>
                  </div>
                </div>
              ))}
            </div>
            {page === 0 && (
              <small className="muted">
                {t("累计自已采集历史持续增加，不随每日切换或明细清理归零。", "Totals accumulate recorded history and survive daily rollover and detail expiry.")}
              </small>
            )}
            {totals.uncertain && (
              <div className="notice">
                {t(
                  "统计含离线后补记流量，发生时间无法精确还原。",
                  "Includes recovered traffic whose exact transfer times are unknown.",
                )}
              </div>
            )}
          </>
        )}
        {page === 0 && (
          <>
            <section className="card">
              <div className="section-head">
                <h2>{t("实时速率", "Live transfer rate")}</h2>
                <div className="plot-controls">
                  <select
                    aria-label={t("速率方向", "Rate direction")}
                    value={speedMetric}
                    onChange={(e) => setSpeedMetric(e.target.value)}
                  >
                    <option value="up">{t("上传", "Upload")}</option>
                    <option value="down">{t("下载", "Download")}</option>
                  </select>
                  <select
                    aria-label={t("实时曲线范围", "Live history range")}
                    value={speedWindow}
                    onChange={(e) => setSpeedWindow(Number(e.target.value))}
                  >
                    {[
                      [300, "5分钟", "5 min"],
                      [900, "15分钟", "15 min"],
                      [1800, "30分钟", "30 min"],
                      [3600, "1小时", "1 h"],
                      [10800, "3小时", "3 h"],
                      [21600, "6小时", "6 h"],
                      [43200, "12小时", "12 h"],
                      [86400, "24小时", "24 h"],
                    ].map(([v, zh, en]) => (
                      <option key={v} value={v}>
                        {t(zh, en)}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
              {(data.siteSpeeds || []).length ? (
                <Plot
                  data={
                    demo
                      ? data.siteSpeeds.filter(
                          (row) => row.time >= Date.now() / 1000 - speedWindow,
                        )
                      : data.siteSpeeds
                  }
                  sites={c.sites}
                  dark={dark}
                  metric={speedMetric}
                  rangeSeconds={speedWindow}
                />
              ) : (
                <div className="empty">
                  {t(
                    "连接下载器后开始记录各站点速率；升级前的总速率不会伪装成站点历史。",
                    "Connect a downloader to see live transfer rates",
                  )}
                </div>
              )}
            </section>
            <section className="card downloading-card">
              <div className="section-head">
                <div>
                  <h2>{t("正在下载", "Downloading")}</h2>
                  <small>
                    {downloadTotal} {t("个种子", "torrents")} · ↓{" "}
                    {bytes(
                      data.downloadRate ??
                        downloading.reduce(
                          (sum, row) => sum + (row.down || 0),
                          0,
                        ),
                      true,
                    )}
                  </small>
                </div>
              </div>
              {!downloading.length ? (
                <div className="empty">
                  {t("暂无正在下载的种子", "No torrents downloading")}
                </div>
              ) : (
                <div className="table-wrap">
                  <table className="download-table">
                    <thead>
                      <tr>
                        {[
                          t("种子名称", "Torrent name"),
                          t("当前下载", "Download rate"),
                          t("进度", "Progress"),
                          t("下载器", "Downloader"),
                          t("站点", "Site"),
                          t("当前上传", "Upload rate"),
                        ].map((label) => (
                          <th key={label}>{label}</th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {downloading.map((row) => (
                        <tr key={row.key}>
                          <td title={row.name}>
                            <span className="download-name">{row.name}</span>
                          </td>
                          <td className="down">{bytes(row.down, true)}</td>
                          <td>
                            {typeof row.progress === "number" ? (
                              <div className="download-progress">
                                <progress
                                  max="1"
                                  value={Math.max(0, Math.min(1, row.progress))}
                                />
                                <small>
                                  {(
                                    Math.max(0, Math.min(1, row.progress)) * 100
                                  ).toFixed(1)}
                                  %
                                </small>
                              </div>
                            ) : (
                              "—"
                            )}
                          </td>
                          <td>
                            {c.clients.find((cl) => cl.id === row.client)
                              ?.name || "—"}
                          </td>
                          <td>{siteName(row.site)}</td>
                          <td className="up">{bytes(row.up, true)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
              {downloadTotal > 50 && (
                <div className="pagination">
                  <button
                    disabled={downloadCurrent === 0}
                    onClick={() => setDownloadPage(downloadCurrent - 1)}
                  >
                    {t("上一页", "Previous")}
                  </button>
                  <small>
                    {downloadCurrent + 1} / {Math.ceil(downloadTotal / 50)}
                  </small>
                  <button
                    disabled={(downloadCurrent + 1) * 50 >= downloadTotal}
                    onClick={() => setDownloadPage(downloadCurrent + 1)}
                  >
                    {t("下一页", "Next")}
                  </button>
                </div>
              )}
              <p className="hint">
                {t(
                  "包含正在下载或等待数据的运行中任务；暂停和仅做种任务不显示。",
                  "Includes running downloads waiting for data. Paused and seeding-only tasks are excluded.",
                )}
              </p>
            </section>
            <section className="card allocation-card">
              <div className="section-head">
                <div>
                  <h2>{t("动态带宽分配", "Dynamic bandwidth")}</h2>
                  {guard}
                  <div className="allocation-summary">
                    <span>
                      {t("活跃种子", "Active torrents")}{" "}
                      <strong>
                        {data.activeSummary?.count ?? torrentCount}
                      </strong>
                    </span>
                    <span>
                      {t("当前上传", "Current upload")}{" "}
                      <strong className="up">
                        {bytes(data.activeSummary?.up ?? speed, true)}
                      </strong>
                    </span>
                    {data.activeSummary && (
                      <span>
                        {t("已分配额度", "Allocated allowance")}{" "}
                        <strong>{bytes(data.activeSummary.limit, true)}</strong>
                      </span>
                    )}
                  </div>
                </div>
              </div>
              {torrents}
              <p className="hint">
                <Info size={15} />
                {c.uploadConfirmSeconds > 0
                  ? t(
                      `累计上传字节在连续采样间隔内增长，确认至少 ${c.uploadConfirmSeconds} 秒后参与分配。`,
                      `Torrents qualify after at least ${c.uploadConfirmSeconds} seconds of growth across consecutive upload samples.`,
                    )
                  : t(
                      "显示有上传连接或正在上传的种子，包括待分配和操作失败项；自动策略由各下载器独立控制。",
                      "Shows torrents with upload connections or actual uploads, including pending and failed operations. Each downloader controls its own strategy.",
                    )}
              </p>
            </section>
          </>
        )}
        {page === 1 && (
          <>
            <section className="card">
              <div className="section-head">
                <h2>{t("流量趋势", "Traffic history")}</h2>
                <select
                  aria-label={t("流量方向", "Traffic direction")}
                  value={trafficMetric}
                  onChange={(e) => setTrafficMetric(e.target.value)}
                >
                  <option value="up">{t("上传", "Upload")}</option>
                  <option value="down">{t("下载", "Download")}</option>
                </select>
              </div>
              {buckets.length ? (
                <Plot
                  data={data.buckets}
                  bars
                  sites={c.sites}
                  metric={trafficMetric}
                  rangeSeconds={data.end - data.start}
                  dark={dark}
                />
              ) : (
                <div className="empty">
                  {t(
                    "该时段暂无新增流量",
                    "No new traffic recorded for this period",
                  )}
                </div>
              )}
            </section>
            <section className="card">
              {siteTable}
              <p className="hint">
                <Info size={15} />
                {t(
                  "统计按实际记录的站点归属汇总。名称、权重及域名规则请在下载管理的站点策略中调整。",
                  "Totals retain their recorded site attribution. Edit names, weights and rules under Download management → Site policies.",
                )}
              </p>
            </section>
          </>
        )}
        {page === 10 && (
          <section className="card">
            <h2>{t("下载器流量", "Downloader traffic")}</h2>
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>{t("下载器", "Downloader")}</th>
                    <th>{t("上传", "Upload")}</th>
                    <th>{t("下载", "Download")}</th>
                  </tr>
                </thead>
                <tbody>
                  {(data.downloaderStats || []).map((row) => (
                    <tr key={row.id}>
                      <td>
                        {c.clients.find((cl) => cl.id === row.id)?.name ||
                          t("历史下载器", "Historical downloader")}
                        {row.uncertain && (
                          <small>
                            {t("含补记流量", "Includes uncertain intervals")}
                          </small>
                        )}
                      </td>
                      <td>{bytes(row.up)}</td>
                      <td>{bytes(row.down)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {!data.downloaderStats?.length && (
                <div className="empty">
                  {t(
                    "该时段暂无新增流量",
                    "No new traffic recorded for this period",
                  )}
                </div>
              )}
            </div>
          </section>
        )}
        {page === 2 && (
          <>
            <div className="toolbar">
              {guard}
              <span className="grow" />
              <button
                className="primary"
                onClick={() =>
                  open("pool", {
                    id: id(),
                    name: t("主出口", "Main uplink"),
                    budget: 6250,
                    enabled: true,
                  })
                }
              >
                <Plus />
                {t("添加出口", "Add outlet")}
              </button>
            </div>
            <div className="notice">
              {t(
                "不同出口使用不同带宽池。默认使用独立守护，主容器停止后仍能恢复系统限速；守护或下载器也离线时需等恢复连接。最低额度不代表实际速度。",
                "Use separate uplink pools. The default independent guardian can recover limits while the main container is stopped. Recovery waits if the guardian or downloader is unreachable. Allowances do not guarantee actual speed.",
              )}
            </div>
            {c.pools.map((p) => (
              <section className="card" key={p.id}>
                <div className="section-head">
                  <div>
                    <h2>{p.name}</h2>
                    <span className={"badge " + (!p.enabled ? "neutral" : "")}>
                      {p.enabled
                        ? t("已启用", "Enabled")
                        : t("只读监控", "Monitoring")}
                    </span>
                  </div>
                  <button
                    onClick={() =>
                      open("pool", { ...p, budget: rateInput(p.budget) })
                    }
                  >
                    <GearSix />
                    {t("配置", "Configure")}
                  </button>
                </div>
                <div className="stat-label">
                  {t("总上传额度", "Upload budget")}
                </div>
                <h1>
                  {bytes(data.outletBudgets?.[p.id]?.budget ?? p.budget, true)}
                </h1>
                <p className="hint">
                  {data.outletBudgets?.[p.id]?.night
                    ? t("当前：夜间预算", "Active: night budget")
                    : t("当前：日间/基础预算", "Active: day/base budget")}{" "}
                  · {c.timezone}
                </p>
                {p.schedule?.enabled && (
                  <p className="hint">
                    {t("日间", "Day")} {bytes(p.budget, true)} ·{" "}
                    {t("夜间", "Night")} {bytes(p.schedule.nightBudget, true)} ·{" "}
                    {p.schedule.start} — {p.schedule.end}
                  </p>
                )}
                <p className="subtitle">
                  {c.clients
                    .filter((cl) => cl.pool === p.id)
                    .map((cl) => cl.name)
                    .join(" · ") ||
                    t("尚未分配下载器", "No downloaders assigned")}
                </p>
              </section>
            ))}
          </>
        )}
        {page === 8 && (
          <>
            {" "}
            <section className="card">
              <div className="section-head">
                <h2>{t("站点权重与额度", "Site weights & allowances")}</h2>
                <div className="toolbar">
                  <button onClick={() => open("match", { url: "" })}>
                    {t("测试匹配", "Test rules")}
                  </button>
                  <button className="primary" onClick={newSite}>
                    <Plus />
                    {t("添加站点", "Add site")}
                  </button>
                </div>
              </div>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      {[
                        t("站点", "Site"),
                        t("权重", "Weight"),
                        t("最低额度", "Minimum"),
                        t("单种上限", "Maximum"),
                      ].map((x) => (
                        <th key={x}>{x}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {c.sites.map((s) => (
                      <tr
                        key={s.id}
                        className="action-row"
                        onClick={() => editSite(s)}
                      >
                        <td className="name">
                          {s.name}
                          <small>
                            {s.catalogId
                              ? t("站点库识别", "Catalog match")
                              : s.automatic
                                ? t("域名发现", "Domain discovery")
                                : t("手动规则", "Manual rule")}
                          </small>
                        </td>
                        <td>{s.weight}</td>
                        <td>{bytes(s.minimum, true)}</td>
                        <td>{s.maximum ? bytes(s.maximum, true) : "—"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
            <section className="card">
              <div className="section-head">
                <h2>{t("种子归属", "Torrent ownership")}</h2>
                <input
                  className="search"
                  placeholder={t("搜索种子名称", "Search torrents")}
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              {torrents}
            </section>
          </>
        )}
        {page === 11 && (
          <>
            <section className="card">
              <div className="section-head">
                <h2>{t("故障诊断", "Diagnostics")}</h2>
                <select
                  aria-label={t("筛选诊断类型", "Filter diagnostics")}
                  value={diagnosticFilter}
                  onChange={(e) => setDiagnosticFilter(e.target.value)}
                >
                  <option value="">{t("全部类型", "All types")}</option>
                  <option value="diag-sampling">{t("下载器采样", "Downloader sampling")}</option>
                  <option value="diag-tracker">{t("Tracker 元数据", "Tracker metadata")}</option>
                  <option value="diag-statistics">{t("流量统计写入", "Traffic statistics writes")}</option>
                  <option value="diag-speed">{t("速率历史写入", "Speed history writes")}</option>
                  <option value="diag-storage">{t("控制器存储", "Controller storage")}</option>
                </select>
              </div>
              {data.diagnosticsUnavailable && (
                <p role="status">{t("历史诊断暂无法读取，可查看容器日志。", "Diagnostic history is temporarily unavailable. Check the container logs.")}</p>
              )}
              {(() => {
                const entries = (data.diagnostics || []).filter(
                  (entry) => !diagnosticFilter || entry.kind === diagnosticFilter,
                );
                const categories = {
                  "diag-sampling": t("下载器采样", "Downloader sampling"),
                  "diag-tracker": t("Tracker 元数据", "Tracker metadata"),
                  "diag-statistics": t("流量统计写入", "Traffic statistics writes"),
                  "diag-speed": t("速率历史写入", "Speed history writes"),
                  "diag-storage": t("控制器存储", "Controller storage"),
                };
                return entries.length ? (
                  <div className="table-wrap">
                    <table>
                      <thead>
                        <tr>
                          <th>{t("时间", "Time")}</th>
                          <th>{t("类型", "Type")}</th>
                          <th>{t("来源", "Source")}</th>
                          <th>{t("详情", "Details")}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {entries.map((entry, index) => (
                          <tr key={index}>
                            <td>{new Date(entry.time * 1000).toLocaleString()}</td>
                            <td>{categories[entry.kind] || entry.kind}</td>
                            <td>{c.clients.find((client) => client.id === entry.key)?.name || entry.key || t("主服务", "Controller")}</td>
                            <td>{entry.message === "Recovered" ? t("已恢复", "Recovered") : entry.message}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <div className="empty">{t("暂无匹配的诊断记录", "No matching diagnostic events")}</div>
                );
              })()}
            </section>
            <section className="card">
              <h2>{t("恢复清单", "Recovery queue")}</h2>
              {!data.leases.length ? (
                <div className="empty">
                  {t(
                    "没有需要恢复的系统限速",
                    "No system limits pending recovery",
                  )}
                </div>
              ) : (
                <div className="table-wrap">
                  <table>
                    <tbody>
                      {data.leases.map((v) => (
                        <tr key={v.key}>
                          <td className="name">{v.key}</td>
                          <td>{bytes(v.limit, true)}</td>
                          <td>{v.state}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </section>
            <section className="card">
              <div className="section-head">
                <h2>{t("调速操作", "Allocation activity")}</h2>
                <small>{t("最近 100 条", "Latest 100 events")}</small>
              </div>
              {!data.events.length ? (
                <div className="empty">
                  {t("暂无操作记录", "No activity yet")}
                </div>
              ) : (
                <div className="table-wrap">
                  <table>
                    <tbody>
                      {data.events.map((v, i) => (
                        <tr key={i}>
                          <td>{new Date(v.time * 1000).toLocaleString()}</td>
                          <td>{v.kind}</td>
                          <td className="name">{v.message}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </section>
          </>
        )}
        {page === 3 && (
          <>
            <div className="toolbar">
              <p className="subtitle grow">
                {t(
                  "支持多个 qBittorrent 与 Transmission 实例",
                  "Connect multiple qBittorrent and Transmission instances",
                )}
              </p>
              <button className="primary" onClick={newClient}>
                <Plus />
                {t("添加下载器", "Add downloader")}
              </button>
            </div>
            <section className="card">{clients}</section>
            <p className="hint">
              <Info size={15} />
              {t(
                "连接凭据加密保存；测试使用当前表单，不要求先保存。",
                "Credentials are encrypted. Test the current form before saving.",
              )}
            </p>
          </>
        )}
        {page === 5 && (
          <>
            <section className="card">
              <div className="section-head">
                <h2>{t("Webhook 推送通道", "Webhook channel")}</h2>
                <button
                  className="primary"
                  onClick={() =>
                    open("notify", {
                      method: "POST",
                      bodyType: "json",
                      bodyTemplate:
                        '{"title":"{{title}}","content":"{{content}}"}',
                      reportTime: "08:00",
                      ...c.notify,
                      headers: "",
                    })
                  }
                >
                  {t("配置通知", "Configure notifications")}
                </button>
              </div>
              <p>
                {c.notify?.enabled
                  ? t("已启用", "Enabled")
                  : t("未启用", "Disabled")}
              </p>
              <p className="hint">
                {t(
                  "沿用图床的 URL、方法、JSON 请求头和 JSON/Text 模板；支持 title、content、text、event、time、fields。请求头留空保留，测试使用当前草稿。",
                  "Uses Gallery-style URL, method, JSON headers and JSON/Text templates. Variables: title, content, text, event, time, fields. Blank headers preserve saved values. Tests use the draft.",
                )}
              </p>
            </section>
            <section className="card">
              <h2>{t("报告与告警", "Reports & alerts")}</h2>
              <p>
                {t(
                  "按系统时区发送：日报报告前一天，周报在周一报告上一周，月报在每月1日报告上个月。告警包括出口最低额度超预算、下载器离线和清空限速待重试。同一持续异常只推送一次；解除后取消待重试，再次发生才重新推送。",
                  "Uses the configured timezone. Daily reports cover yesterday; Monday reports cover last week; first-of-month reports cover last month. Alerts cover minimums above budget, offline downloaders and pending clearance, once per continuous incident; resolved retries are cancelled and recurrence notifies again.",
                )}
              </p>
              <p className="hint">
                {t(
                  "失败最多尝试4次，间隔1/5/15分钟，任务和结果持久保存；响应丢失时外部服务可能收到重复通知。",
                  "Up to four attempts with 1/5/15-minute backoff, persisted across restart. A lost response can cause duplicate delivery.",
                )}
              </p>
            </section>
            <section className="card">
              <div className="section-head">
                <h2>{t("投递记录", "Delivery history")}</h2>
                <button
                  onClick={() =>
                    api("notification-jobs")
                      .then(setNotificationJobs)
                      .catch((e) => setError(e.message))
                  }
                >
                  {t("刷新记录", "Refresh")}
                </button>
              </div>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>{t("类型", "Type")}</th>
                      <th>{t("状态", "State")}</th>
                      <th>HTTP</th>
                      <th>{t("次数", "Attempts")}</th>
                      <th>{t("时间", "Time")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {notificationJobs.map((j, i) => (
                      <tr key={i}>
                        <td>{j.kind}</td>
                        <td>
                          {j.state}
                          <small>{j.error}</small>
                        </td>
                        <td>{j.status || "—"}</td>
                        <td>{j.attempts}</td>
                        <td>{new Date(j.time * 1000).toLocaleString()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {!notificationJobs.length && (
                  <div className="empty">
                    {t("暂无投递记录", "No deliveries yet")}
                  </div>
                )}
              </div>
            </section>
          </>
        )}
        {page === 6 && (
          <section className="card">
            <h2>{t("完整备份与恢复", "Full backup & restore")}</h2>
            <p>
              {t(
                "备份包包含账号、下载器与站点配置、出口、通知设置、统计、运行记录和应用密钥，可导入另一处流量台。备份含敏感信息，请妥善保存。",
                "Includes accounts, downloaders, sites, outlets, notification settings, statistics, logs and encryption key. Import into another Torrent Flow deployment. The archive contains sensitive data; keep it private.",
              )}
            </p>
            <div className="toolbar">
              <button
                className="primary"
                disabled={busy || demo}
                onClick={() => {
                  location.href = "/api/backup";
                }}
              >
                <DownloadSimple />
                {t("下载完整备份包", "Download full backup")}
              </button>
            </div>
            <hr />
            <h3>{t("从备份包恢复", "Restore from archive")}</h3>
            <p className="hint">
              {t(
                "先停止各下载器策略并等待限速清空。导入校验通过后才覆盖，保留服务端回退备份；需要用备份中的账号重新登录，调速保持暂停。部署端口、域名和挂载由新环境配置。",
                "Stop each downloader strategy and wait for clearance. Archives are validated before replacement; a rollback archive is retained. Sign in with the backup account afterwards; allocation remains paused. Configure ports, domains and mounts in the new environment.",
              )}
            </p>
            <input
              ref={backupInput}
              hidden
              type="file"
              accept=".zip,application/zip"
              onChange={(e) => setBackupFile(e.target.files?.[0] || null)}
            />
            <div className="toolbar">
              <button
                disabled={busy || demo}
                onClick={() => backupInput.current?.click()}
              >
                {t("选择备份包", "Choose backup file")}
              </button>
              <span className="backup-filename">
                {backupFile?.name || t("尚未选择备份包", "No backup selected")}
              </span>
              <button
                className="primary"
                disabled={busy || !backupFile || demo}
                onClick={restoreBackup}
              >
                {t("导入并恢复", "Import & restore")}
              </button>
            </div>
          </section>
        )}
        {page === 7 && (
          <section className="card about-policy">
            <h2>Torrent Flow · {t("流量台", "Traffic monitor")}</h2>
            <p>
              v{data.version} · MIT License ·{" "}
              <a
                href="https://github.com/Stylle/torrent-flow"
                target="_blank"
                rel="noreferrer"
              >
                GitHub
              </a>
            </p>
            <h3>{t("统计与自动策略", "Accounting and allocation")}</h3>
            <p>
              {t(
                "已启用的下载器持续统计，每个下载器独立启停自动策略；停用下载器会停止采集并恢复系统限速。相同出口共同分配预算，活跃站点先按1–10权重分配，再由站内种子平分。最低额度不足时调整，固定最低额度优先于出口预算、单种子上限及达标后站点额度，超出时告警，不降低最低额度。",
                "Enabled downloaders keep monitoring; each independently starts/stops allocation. Disabling a downloader stops collection and restores system-owned limits. Downloaders share an outlet budget. Active sites receive weighted shares (1–10), split equally across torrents. Fixed minimums take precedence over budgets, per-torrent ceilings and post-target site allowances, with distinct warnings.",
              )}
            </p>
            <h3>{t("停止与恢复", "Stop and recovery")}</h3>
            <p>
              {t(
                "停止策略会清空该下载器所有种子级上传限速，包括手工限速；不改全局限速。种子闲置后按采集设置释放系统限速。主服务掉线由独立守护恢复；守护、下载器或整机也离线时需等待恢复连接。",
                "Stopping clears every torrent-level upload limit on that downloader, including manual limits, without changing global limits. Idle release defaults to15 seconds. An independent guardian recovers a stopped controller; recovery waits if the worker, downloader or host is unreachable.",
              )}
            </p>
            <h3>{t("图表与数据", "Charts and data")}</h3>
            <p>
              {t(
                "速率按站点记录，最多保留24小时；长时间窗口会聚合为平均速率以控制占用。统计保留一年，柱状图按站点堆叠实际字节数，提示各站点占比；不是把每根柱都强制拉成100%。离线补记不能精确还原发生时间，历史归属不重写。",
                "Per-site rate history retains24 hours; longer windows use averaged buckets. Traffic retains one year. Bars stack actual bytes by site and show percentage shares; they are not normalized to100%. Recovered offline traffic has uncertain timing; past attribution is preserved.",
              )}
            </p>
            <h3>{t("备份与通知", "Backups and notifications")}</h3>
            <p>
              {t(
                "备份包含账号、配置、统计和密钥；导入前停止策略并等待清空，恢复后重新登录且策略暂停。通知支持日周月报告与告警，失败有限重试，响应丢失仍可能重复。默认6250 KB/s为6.25 MB/s，最低额度默认每种子50 KB/s；额度不是实际速度保证。",
                "Backups contain accounts, settings, statistics and keys. Stop strategies and wait for clearance before import; sign in again afterwards with allocation paused. Reports and alerts have bounded retries and may duplicate after a lost response. Default6250 KB/s equals6.25 MB/s; the per-torrent minimum defaults to50 KB/s. Allowances do not guarantee actual speed.",
              )}
            </p>
          </section>
        )}
        {page === 9 && (
          <section className="card">
            <div className="section-head">
              <h2>{t("自动站点识别", "Automatic site recognition")}</h2>
              <button
                disabled={busy}
                onClick={() => catalogAction({ update: true })}
              >
                <ArrowClockwise />
                {t("立即更新", "Update now")}
              </button>
            </div>
            <p className="subtitle">PT-depiler · MIT</p>
            <label className="checkbox">
              <input
                type="checkbox"
                checked={data.catalog?.auto !== false}
                disabled={busy}
                onChange={(e) => catalogAction({ auto: e.target.checked })}
              />
              {t("每天自动检查站点库更新", "Check for catalog updates daily")}
            </label>
            <dl className="catalog-status">
              <dt>{t("站点／域名", "Sites / domains")}</dt>
              <dd>
                {data.catalog?.sites ?? 0} / {data.catalog?.domains ?? 0}
              </dd>
              <dt>{t("数据版本", "Data version")}</dt>
              <dd>
                {data.catalog?.version?.slice(0, 12) ||
                  t("内置版本", "Bundled")}
              </dd>
              <dt>{t("最近检查", "Last checked")}</dt>
              <dd>
                {data.catalog?.checked
                  ? new Date(data.catalog.checked * 1000).toLocaleString()
                  : t(
                      "尚未检查，使用内置库",
                      "Not checked; using bundled data",
                    )}
              </dd>
              <dt>{t("最近更新", "Last updated")}</dt>
              <dd>
                {data.catalog?.updated
                  ? new Date(data.catalog.updated * 1000).toLocaleString()
                  : "—"}
              </dd>
            </dl>
            {data.catalog?.error && (
              <p className="notice" role="status">
                {data.catalog.error}
              </p>
            )}
            <p className="hint">
              {t(
                "优先使用手动规则。自动识别只匹配公开域名，忽略 passkey；同站新域名合并，已有分组、名称、权重与历史统计保留。冲突或未收录的域名保持原分类。更新失败继续使用当前库。",
                "Manual rules take priority. Only public domains are matched; passkeys are ignored. New aliases are grouped, while existing groups, names, weights and history remain. Conflicting or unknown domains keep their classification. Failed updates retain the current catalog.",
              )}
            </p>
            <a
              href="https://github.com/pt-plugins/PT-depiler"
              target="_blank"
              rel="noreferrer"
            >
              {t("查看数据来源", "View source")}
            </a>
          </section>
        )}
        {page === 4 && (
          <>
            <div className="setting-grid">
              <section className="card">
                <h2>{t("外观与语言", "Appearance & language")}</h2>
                <p className="subtitle">
                  {t("仅保存在此设备", "Saved on this device only")}
                </p>
                <div className="toolbar" style={{ marginTop: 20 }}>
                  {controls}
                </div>
              </section>
              <section className="card">
                <h2>{t("系统设置", "System settings")}</h2>
                {[
                  [
                    t("时区与采集频率", "Timezone & sampling"),
                    () =>
                      open("preferences", {
                        timezone: c.timezone,
                        pollSeconds: c.pollSeconds,
                        idleSeconds: c.idleSeconds,
                        uploadConfirmSeconds: c.uploadConfirmSeconds ?? 0,
                      }),
                  ],
                  [
                    t("修改登录密码", "Change password"),
                    () => open("password", { current: "", password: "" }),
                  ],
                ].map(([label, fn]) => (
                  <button className="settings-action" key={label} onClick={fn}>
                    {label}
                    <ArrowRight />
                  </button>
                ))}
              </section>
            </div>
            <p className="footnote">
              Torrent Flow v{data.version} · MIT License
              <br />
              {t(
                "流量最长保留一年。备份需完整保留数据目录，含 master.key。",
                "Traffic is retained for one year. Back up the entire data directory, including master.key.",
              )}
            </p>
          </>
        )}
      </main>
      {toast && (
        <div className="toast" role="status">
          {toast}
        </div>
      )}
      {modal && (
        <div className="backdrop">
          <section
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-label={t("配置", "Configuration")}
          >
            <div className="modal-head">
              <h2>
                {
                  {
                    notify: t("Webhook 与推送规则", "Webhook & delivery rules"),
                    client: t("下载器连接", "Downloader connection"),
                    site: t("站点配置", "Site settings"),
                    pool: t("带宽池", "Bandwidth pool"),
                    torrent: t("种子设置", "Torrent settings"),
                    password: t("修改密码", "Change password"),
                    preferences: t("采集设置", "Sampling settings"),
                    match: t("测试站点匹配", "Test site rules"),
                  }[modal]
                }
              </h2>
              <button
                className="icon"
                aria-label={t("关闭", "Close")}
                onClick={() => setModal(null)}
                disabled={busy}
              >
                <X />
              </button>
            </div>
            <form onSubmit={submit}>
              <div className="fields">
                {modal === "notify" && (
                  <>
                    {toggle(
                      "enabled",
                      "启用 Webhook 通知",
                      "Enable Webhook notifications",
                    )}
                    {field("url", "Webhook URL", "Webhook URL", "url", {
                      required: true,
                    })}
                    {select("method", "请求方法", "Method", [
                      ["POST", "POST"],
                      ["PUT", "PUT"],
                      ["PATCH", "PATCH"],
                    ])}
                    <label>
                      {t("自定义请求头（JSON）", "Custom headers (JSON)")}
                      <textarea
                        value={form.headers || ""}
                        placeholder={
                          form.headersStored
                            ? t(
                                "请求头已保存，留空保留",
                                "Saved headers; blank preserves",
                              )
                            : '{"Authorization":"Bearer ..."}'
                        }
                        onChange={(e) =>
                          setForm({ ...form, headers: e.target.value })
                        }
                      />
                    </label>
                    {select("bodyType", "请求体格式", "Body format", [
                      ["json", "JSON"],
                      ["text", "Text"],
                    ])}
                    <label>
                      {t("请求体模板", "Body template")}
                      <textarea
                        rows={5}
                        value={form.bodyTemplate || ""}
                        onChange={(e) =>
                          setForm({ ...form, bodyTemplate: e.target.value })
                        }
                      />
                    </label>
                    <small>
                      {t(
                        "默认发送 title 和 content；支持 {{title}}、{{content}}、{{text}}、{{event}}、{{time}}、{{fields}}。",
                        "Defaults to title and content. Supports {{title}}, {{content}}, {{text}}, {{event}}, {{time}}, {{fields}}.",
                      )}
                    </small>
                    <h3>{t("报告与告警开关", "Report & alert switches")}</h3>
                    {toggle("daily", "发送日报", "Daily report")}
                    {toggle("weekly", "发送周报", "Weekly report")}
                    {toggle("monthly", "发送月报", "Monthly report")}
                    {toggle("alerts", "发送异常告警", "Operational alerts")}
                    {field(
                      "reportTime",
                      "报告发送时间（系统时区）",
                      "Report time (system timezone)",
                      "time",
                    )}
                  </>
                )}
                {modal === "client" && (
                  <>
                    {field("name", "名称", "Name", "text", { required: true })}
                    {select("kind", "类型", "Type", [
                      ["qb", "qBittorrent"],
                      ["tr", "Transmission"],
                    ])}
                    {field("url", "连接地址", "Address", "text", {
                      required: true,
                      placeholder:
                        form.kind === "qb"
                          ? "http://192.168.1.2:8080"
                          : "http://192.168.1.2:9091/transmission/rpc",
                    })}
                    {field("user", "账号", "Username")}
                    {field(
                      "password",
                      "密码（留空保留已保存值）",
                      "Password (blank keeps saved value)",
                      "password",
                    )}
                    {select("pool", "所属带宽池", "Bandwidth pool", [
                      ...c.pools.map((p) => [p.id, p.name]),
                    ])}
                    <small>
                      {t(
                        "所有已添加的下载器持续采集统计，自动调速可在下载器列表单独启动或停止。",
                        "Monitoring remains active for every configured downloader. Start or stop rate allocation independently in the downloader list.",
                      )}
                    </small>
                  </>
                )}
                {modal === "site" && (
                  <>
                    {field("name", "站点名称", "Site name", "text", {
                      required: true,
                    })}
                    <small>
                      {t(
                        "域名自动识别，通常只需改名。默认权重 10，可降到 1 减少该站上传；每个活跃种子最低 50 KB/s。",
                        "Tracker domains are discovered automatically. Default site weight is 10; lower it to reduce upload priority. Each active torrent has a fixed 50 KB/s minimum.",
                      )}
                    </small>
                    <details className="advanced-fields" open={!form.automatic}>
                      <summary>
                        {t(
                          "高级匹配（通常无需修改）",
                          "Advanced matching (usually unchanged)",
                        )}
                      </summary>
                      <label>
                        {t("匹配域名（每行一个）", "Domains (one per line)")}
                        <textarea
                          value={form.domainText}
                          placeholder={"a.example\nb.example"}
                          onChange={(e) =>
                            setForm({ ...form, domainText: e.target.value })
                          }
                        />
                      </label>
                      {field(
                        "regex",
                        "正则匹配（可选，RE2）",
                        "Regex (optional, RE2)",
                      )}
                      <small>
                        {t(
                          "域名规则忽略路径和 passkey；正则匹配完整 Tracker URL。",
                          "Domains ignore paths/passkeys; regex matches the complete tracker URL.",
                        )}
                      </small>
                    </details>
                    <small>
                      {t(
                        "先按站点权重分配，再由站内种子平分。最低额度不足时调整；最低额度合计超出口预算时允许超出并警告，不降低最低额度。",
                        "Allocate by site weight, then split equally within each site. Fixed per-torrent minimums take precedence, even above the outlet budget, with a warning.",
                      )}
                    </small>
                    <div className="field-pair">
                      {field(
                        "weight",
                        "站点权重 1–10（默认 10）",
                        "Site weight 1–10 (default 10)",
                        "number",
                        { min: 1, max: 10 },
                      )}
                      {field(
                        "minimum",
                        "每种子最低额度 KB/s（默认 50）",
                        "Minimum per torrent KB/s (default 50)",
                        "number",
                        { min: 0, step: 0.001 },
                      )}
                    </div>
                    {field(
                      "maximum",
                      "每种子最高额度 KB/s（0 无上限）",
                      "Maximum per torrent KB/s (0 = uncapped)",
                      "number",
                      { min: 0, step: 0.001 },
                    )}
                    <small>
                      {t(
                        "最高额度限制每个种子；站点可分配上限为活跃种子数×单种子上限。达标后额度仍为站点总额度。最低额度冲突时保留最低并明确提示，只有最低合计超出出口预算才报出口超额。",
                        "Maximum applies per torrent; site capacity is active torrents × maximum. After-target allowance remains site-wide. Fixed minima prevail; only outlet floor totals above its budget trigger an outlet warning.",
                      )}
                    </small>
                    <div className="field-pair">
                      {field(
                        "dayTarget",
                        "日目标 GB（0 关闭）",
                        "Daily target GB (0 = off)",
                        "number",
                        { min: 0, step: "any" },
                      )}
                      {field(
                        "monthTarget",
                        "月目标 GB（0 关闭）",
                        "Monthly target GB (0 = off)",
                        "number",
                        { min: 0, step: "any" },
                      )}
                    </div>
                    {field(
                      "afterTarget",
                      "达标后站点总额度 KB/s（大于 0）",
                      "Site total after target KB/s (>0)",
                      "number",
                      { min: 0.001, step: 0.001 },
                    )}
                  </>
                )}
                {modal === "pool" && (
                  <>
                    {field("name", "名称", "Name", "text", { required: true })}
                    {field(
                      "budget",
                      "日间/基础上传额度 KB/s（默认 6250）",
                      "Day/base upload allowance KB/s (default 6250)",
                      "number",
                      { required: true, min: 1.024, step: 0.001 },
                    )}
                    <h3>{t("夜间备用预算", "Night budget")}</h3>
                    <label className="checkbox">
                      <input
                        type="checkbox"
                        checked={!!form.schedule?.enabled}
                        onChange={(e) =>
                          setForm({
                            ...form,
                            schedule: {
                              ...form.schedule,
                              enabled: e.target.checked,
                            },
                          })
                        }
                      />
                      {t(
                        "按时间自动切换日夜预算",
                        "Automatically switch day/night budgets",
                      )}
                    </label>
                    <div
                      className="schedule-fields"
                      style={{ opacity: form.schedule?.enabled ? 1 : 0.65 }}
                    >
                      <fieldset disabled={!form.schedule?.enabled}>
                        {field(
                          "nightKB",
                          "夜间上传额度 KB/s",
                          "Night upload allowance KB/s",
                          "number",
                          {
                            min: 1.024,
                            step: "any",
                            required: !!form.schedule?.enabled,
                          },
                        )}
                        <div className="field-pair">
                          <label>
                            {t("从", "From")}
                            <input
                              type="time"
                              value={form.schedule?.start || "23:00"}
                              onChange={(e) =>
                                setForm({
                                  ...form,
                                  schedule: {
                                    ...form.schedule,
                                    start: e.target.value,
                                  },
                                })
                              }
                            />
                          </label>
                          <label>
                            {t("到", "To")}
                            <input
                              type="time"
                              value={form.schedule?.end || "08:00"}
                              onChange={(e) =>
                                setForm({
                                  ...form,
                                  schedule: {
                                    ...form.schedule,
                                    end: e.target.value,
                                  },
                                })
                              }
                            />
                          </label>
                        </div>
                        <div className="weekday-options">
                          {[
                            [1, "周一", "Mon"],
                            [2, "周二", "Tue"],
                            [3, "周三", "Wed"],
                            [4, "周四", "Thu"],
                            [5, "周五", "Fri"],
                            [6, "周六", "Sat"],
                            [0, "周日", "Sun"],
                          ].map(([day, zh, en]) => (
                            <label className="checkbox" key={day}>
                              <input
                                type="checkbox"
                                checked={
                                  form.schedule?.days?.includes(day) || false
                                }
                                onChange={(e) =>
                                  setForm({
                                    ...form,
                                    schedule: {
                                      ...form.schedule,
                                      days: e.target.checked
                                        ? [...(form.schedule?.days || []), day]
                                        : (form.schedule?.days || []).filter(
                                            (d) => d !== day,
                                          ),
                                    },
                                  })
                                }
                              />
                              {t(zh, en)}
                            </label>
                          ))}
                        </div>
                      </fieldset>
                    </div>
                    <small>
                      {t(
                        "默认23:00至次日08:00，每天启用，定时开关默认关闭。按系统时区执行；星期指夜间开始的日期，例如勾选周一会覆盖周一23:00至周二08:00。其余时间使用日间/基础预算，不改变下载器策略开关。",
                        "Defaults to23:00–08:00 every day, scheduling off. Uses the configured timezone. Weekdays identify the start date: Monday covers Monday23:00 through Tuesday08:00. Other times use the base budget; downloader strategy switches are unchanged.",
                      )}
                    </small>
                    <small>
                      {t(
                        "默认出口 6250 KB/s = 6.25 MB/s。下载器默认绑定；可按实际出口修改，由各下载器的策略开关管理调速，统计始终运行。",
                        "Default outlet is 6250 KB/s (6.25 MB/s). Downloaders bind automatically. Edit it for your uplink; each downloader strategy switch controls allocation while monitoring always runs.",
                      )}
                    </small>
                  </>
                )}
                {modal === "torrent" && (
                  <>
                    <p style={{ overflowWrap: "anywhere" }}>{form.name}</p>
                    {select(
                      "site",
                      "统计与调度归属",
                      "Accounting & policy owner",
                      [
                        ["", t("自动匹配", "Automatic matching")],
                        ...c.sites.map((s) => [s.id, s.name]),
                      ],
                    )}
                    <small>
                      {t(
                        "无上传连接后恢复种子不限速；保留下载器全局限制。",
                        "Returns to unlimited when idle; global downloader limits remain.",
                      )}
                    </small>
                  </>
                )}
                {modal === "preferences" && (
                  <>
                    {field(
                      "timezone",
                      "时区（IANA）",
                      "Timezone (IANA)",
                      "text",
                      { required: true },
                    )}
                    {field(
                      "pollSeconds",
                      "采样间隔，秒（5–60）",
                      "Sample interval, seconds (5–60)",
                      "number",
                      { min: 5, max: 60 },
                    )}
                    <small>
                      {t(
                        "推荐 5 秒：及时更新流量与带宽分配；种子特别多时可改为 10–15 秒，减少下载器 API 请求。",
                        "Recommended 5 seconds keeps traffic and allocation current. Use 10–15 seconds for large libraries to reduce API requests.",
                      )}
                    </small>
                    {field(
                      "uploadConfirmSeconds",
                      "持续上传确认，秒（0–300）",
                      "Sustained upload confirmation, seconds (0–300)",
                      "number",
                      { min: 0, max: 300, step: 1, required: true },
                    )}
                    <small>
                      {t(
                        "0 为关闭。采样间隔为 15 秒时可从 15 秒试起。首次看到累计上传增长后开始计时，后续采样要继续增长才会参与调速。",
                        "Set 0 to disable. With 15-second sampling, try 15 seconds. Confirmation starts at the first observed increase in cumulative upload; later samples must keep increasing.",
                      )}
                    </small>
                    {field(
                      "idleSeconds",
                      "无连接释放延迟，秒（10–300）",
                      "Idle release delay, seconds (10–300)",
                      "number",
                      { min: 10, max: 300 },
                    )}
                    <small>
                      {t(
                        "开启持续上传确认时，累计上传停止增长后开始判断闲置；关闭时，按无连接且无上传判断闲置。到期释放系统限速。",
                        "With confirmation enabled, inactivity is based on a stalled upload counter. Otherwise, it requires no connections and no upload speed. Idle system limits are then released.",
                      )}
                    </small>
                  </>
                )}
                {modal === "password" && (
                  <>
                    {field(
                      "current",
                      "当前密码",
                      "Current password",
                      "password",
                      { required: true },
                    )}
                    {field("password", "新密码", "New password", "password", {
                      required: true,
                      minLength: 12,
                    })}
                    <small>
                      {t(
                        "12–72 字节，含大小写字母、数字、符号；修改后所有旧会话失效。",
                        "12–72 bytes with upper/lowercase, number and symbol; all old sessions revoked.",
                      )}
                    </small>
                  </>
                )}
                {modal === "match" && (
                  <>
                    {field(
                      "url",
                      "Tracker URL（不保存）",
                      "Tracker URL (not saved)",
                      "password",
                      { required: true },
                    )}
                    {form.result && (
                      <div className="match-result">
                        {form.result.host} →{" "}
                        {form.result.ambiguous
                          ? t("多个站点冲突", "Multiple sites matched")
                          : siteName(form.result.site)}
                      </div>
                    )}
                  </>
                )}
              </div>
              {error && (
                <p className="error inline-error" role="alert">
                  {error}
                </p>
              )}
              <footer className="modal-footer">
                {["client", "site", "pool"].includes(modal) &&
                  c[
                    { client: "clients", site: "sites", pool: "pools" }[modal]
                  ].some((v) => v.id === form.id) && (
                    <button
                      className="danger"
                      type="button"
                      onClick={remove}
                      disabled={busy}
                    >
                      <Trash />
                      {t("删除", "Delete")}
                    </button>
                  )}
                {modal === "notify" && (
                  <button type="button" onClick={testWebhook} disabled={busy}>
                    {t("测试当前配置", "Test draft")}
                  </button>
                )}
                {modal === "client" && (
                  <button type="button" onClick={testClient} disabled={busy}>
                    <Broadcast />
                    {t("测试连接", "Test connection")}
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => setModal(null)}
                  disabled={busy}
                >
                  {t("取消", "Cancel")}
                </button>
                <button className="primary" disabled={busy}>
                  <FloppyDisk />
                  {modal === "match"
                    ? t("测试", "Test")
                    : t("保存设置", "Save settings")}
                </button>
              </footer>
            </form>
          </section>
        </div>
      )}
    </div>
  );
}
