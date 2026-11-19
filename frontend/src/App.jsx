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
const MB = 1048576,
  GB = 1073741824;
const id = createID;
function bytes(v = 0, speed = false) {
  if (!v) return speed ? "0 B/s" : "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"],
    i = Math.max(
      0,
      Math.min(4, Math.floor(Math.log(Math.abs(v)) / Math.log(1024))),
    );
  return `${(v / 1024 ** i).toLocaleString(undefined, { maximumFractionDigits: i < 2 ? 0 : 2 })} ${units[i]}${speed ? "/s" : ""}`;
}
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
    version: "0.1.2",
    config: {
      revision: 0,
      timezone: "Asia/Shanghai",
      pollSeconds: 5,
      idleSeconds: 15,
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
        key: "qb:" + i,
        name,
        site: "s" + i,
        connections: [3, 2, 0][i],
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
function Plot({ data, bars, dark, labels }) {
  const ref = useRef();
  useEffect(() => {
    const css = getComputedStyle(document.documentElement),
      colors = ["--teal", "--blue"].map((k) => css.getPropertyValue(k).trim()),
      muted = css.getPropertyValue("--muted").trim(),
      line = css.getPropertyValue("--line").trim();
    const chart = new Chart(ref.current, {
      type: bars ? "bar" : "line",
      data: {
        labels: data.map((v) =>
          new Date(v.time * 1000).toLocaleString(
            undefined,
            bars
              ? { month: "numeric", day: "numeric" }
              : { hour: "2-digit", minute: "2-digit" },
          ),
        ),
        datasets: ["up", "down"].map((k, i) => ({
          label: labels[i],
          data: data.map((v) => v[k]),
          borderColor: colors[i],
          backgroundColor: bars ? colors[i] : colors[i] + "12",
          borderWidth: 2,
          pointRadius: 0,
          tension: 0.2,
          fill: !bars,
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
              label: (c) => `${c.dataset.label}: ${bytes(c.raw, !bars)}`,
            },
          },
        },
        scales: {
          x: {
            grid: { display: false },
            border: { display: false },
            ticks: {
              color: muted,
              maxTicksLimit: 7,
              maxRotation: 0,
              font: { size: 11 },
            },
          },
          y: {
            beginAtZero: true,
            grid: { color: line },
            border: { display: false },
            ticks: {
              color: muted,
              maxTicksLimit: 5,
              font: { size: 11 },
              callback: (v) => bytes(v, !bars),
            },
          },
        },
      },
    });
    return () => chart.destroy();
  }, [data, bars, dark, labels.join()]);
  return (
    <div className="chart">
      <canvas ref={ref} role="img" aria-label={labels.join(" / ")} />
    </div>
  );
}
export function App() {
  const demo = new URLSearchParams(location.search).get("demo") === "1";
  const [data, setData] = useState(demo ? demoData() : null),
    [auth, setAuth] = useState(demo ? "yes" : "loading"),
    [csrf, setCsrf] = useState(""),
    [page, setPage] = useState(0),
    [menu, setMenu] = useState(false),
    [period, setPeriod] = useState("day"),
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
    [search, setSearch] = useState("");
  const t = (zh, en) => (lang === "zh" ? zh : en),
    c = data?.config;
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
      if (r.status === 401) setAuth("no");
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
      const v = await api(`snapshot?period=${period}&date=${date}`);
      setData(v);
    }
  }
  useEffect(() => {
    if (auth !== "yes" || demo) return;
    let alive = true;
    const update = async () => {
      try {
        const v = await api(`snapshot?period=${period}&date=${date}`);
        if (alive) {
          setData(v);
          setError("");
        }
      } catch (e) {
        if (alive) setError(e.message);
      }
    };
    update();
    const timer = setInterval(update, 5000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [auth, period, date]);
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
    return v;
  }, [data]);
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
      c?.sites.find((s) => s.id === k)?.name || t("未分类", "Unclassified"),
    names = [
      t("流量总览", "Overview"),
      t("站点统计", "Sites"),
      t("带宽策略", "Bandwidth"),
      t("下载器", "Downloaders"),
      t("设置", "Settings"),
    ],
    nav = [House, ChartBar, Stack, DownloadSimple, GearSix];
  function open(kind, v = {}) {
    setModal(kind);
    setError("");
    setForm(structuredClone(v));
  }
  function editSite(s) {
    open("site", {
      ...s,
      domainText: s.domains.join("\n"),
      minimum: s.minimum / MB,
      maximum: s.maximum / MB,
      afterTarget: s.afterTarget / MB,
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
      weight: 3,
      minimum: 0,
      maximum: 0,
      dayTarget: 0,
      monthTarget: 0,
      afterTarget: MB,
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
      pool: "",
      enabled: true,
    });
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
    else if (modal === "preferences") {
      next.timezone = form.timezone;
      next.pollSeconds = Number(form.pollSeconds);
      next.idleSeconds = Number(form.idleSeconds);
    } else {
      const field = { client: "clients", site: "sites", pool: "pools" }[modal],
        v = { ...form };
      if (modal === "site") {
        v.domains = v.domainText.split(/[\n,\s]+/).filter(Boolean);
        delete v.domainText;
        for (const k of ["minimum", "maximum", "afterTarget"])
          v[k] = Math.round(Number(v[k] || 0) * MB);
        for (const k of ["dayTarget", "monthTarget"])
          v[k] = Math.round(Number(v[k] || 0) * GB);
        v.weight = Number(v.weight);
      }
      if (modal === "pool") v.budget = Math.round(Number(v.budget) * MB);
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
  async function release() {
    if (
      !confirm(
        t(
          "关闭自动调速，并恢复本系统接管的种子不限速？",
          "Disable control and release all system-managed limits?",
        ),
      )
    )
      return;
    setBusy(true);
    try {
      if (!demo) await api("release", {});
      setToast(t("已恢复系统接管限速", "System limits released"));
      await refresh();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  async function testClient() {
    setBusy(true);
    try {
      const v = demo ? { torrents: 3 } : await api("test-client", form);
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
      <div className="group">
        <button
          className={lang === "zh" ? "active" : ""}
          onClick={() => setLang("zh")}
        >
          中
        </button>
        <button
          className={lang === "en" ? "active" : ""}
          onClick={() => setLang("en")}
        >
          EN
        </button>
      </div>
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
                    name: t("未分类", "Unclassified"),
                    weight: "—",
                    domains: [],
                  },
                ]
              : []),
          ].map((s) => (
            <tr
              className="action-row"
              key={s.id}
              onClick={() => s.id && editSite(s)}
            >
              <td className="name">
                <span className="dot" />
                {s.name}
                {page === 1 && <small>{s.domains.join(" · ")}</small>}
              </td>
              <td>{bytes(totals.sites[s.id]?.up)}</td>
              <td>{bytes(totals.sites[s.id]?.down)}</td>
              <td className="hide-phone">
                {bytes(
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
            "添加站点规则，自动归类域名",
            "Add site rules to classify tracker domains",
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
                <span className={"dot " + (!s?.online ? "offline" : "")} />
                {s?.online
                  ? t("运行中", "Online")
                  : cl.enabled
                    ? t("连接待检查", "Check connection")
                    : t("未启用", "Disabled")}
              </small>
              {page === 3 && s?.error && <small>{s.error}</small>}
            </div>
            <div className="speeds">
              <span className="up">↑ {bytes(s?.up, true)}</span>
              <span className="down">↓ {bytes(s?.down, true)}</span>
            </div>
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
  const torrents = (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{t("种子", "Torrent")}</th>
            <th className="hide-phone">{t("站点", "Site")}</th>
            <th>{t("上传连接", "Connections")}</th>
            <th>{t("当前限速", "Limit")}</th>
            <th>{t("状态", "State")}</th>
          </tr>
        </thead>
        <tbody>
          {data?.torrents
            .filter(
              (x) =>
                !search || x.name.toLowerCase().includes(search.toLowerCase()),
            )
            .map((x) => (
              <tr
                key={x.key}
                className="action-row"
                onClick={() =>
                  open("torrent", {
                    key: x.key,
                    name: x.name,
                    site: x.site,
                    weight: c.overrides[x.key]?.weight || 1,
                    manage: c.overrides[x.key]?.manage || false,
                  })
                }
              >
                <td className="name">
                  <span className="torrent-name">{x.name}</span>
                  {x.ambiguous && (
                    <small>
                      {t(
                        "多个站点冲突，请指定归属",
                        "Multiple sites; select an owner",
                      )}
                    </small>
                  )}
                </td>
                <td className="hide-phone">{siteName(x.site)}</td>
                <td>{x.connections}</td>
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
              </tr>
            ))}
        </tbody>
      </table>
      {!data?.torrents.length && (
        <div className="empty">
          {t(
            "尚未发现种子，连接下载器后自动采集",
            "No torrents yet. Connect a downloader to begin.",
          )}
        </div>
      )}
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
        <button
          className="backdrop-nav"
          aria-label={t("收起菜单", "Close navigation")}
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
        <nav className="nav">
          {names.map((name, i) => {
            const I = nav[i];
            return (
              <button
                key={i}
                className={page === i ? "active" : ""}
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
            onClick={() => setMenu(!menu)}
          >
            <List size={22} />
          </button>
          <div className="page-title">
            <h1>{names[page]}</h1>
          </div>
          <div className="header-filters">
            {page < 2 && periods}
            {page < 2 && (
              <input
                className="date"
                type="date"
                aria-label={t("统计日期", "Report date")}
                value={date}
                onChange={(e) => e.target.value && setDate(e.target.value)}
              />
            )}
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
        {error && !modal && (
          <div className="notice error" role="alert">
            {error}
          </div>
        )}
        {data.warnings?.map((v) => (
          <div className="notice" key={v}>
            {v}
          </div>
        ))}
        {page < 2 && (
          <>
            <div
              className={
                "toolbar filters " + (page === 0 ? "overview-filters" : "")
              }
            >
              {periods}
              <input
                className="date"
                type="date"
                aria-label={t("统计日期", "Report date")}
                value={date}
                onChange={(e) => e.target.value && setDate(e.target.value)}
              />
              <span className="grow" />
              {page === 1 && (
                <>
                  <button onClick={() => open("match", { url: "" })}>
                    <Link />
                    {t("测试匹配", "Test rules")}
                  </button>
                  <button className="primary" onClick={newSite}>
                    <Plus />
                    {t("添加站点", "Add site")}
                  </button>
                </>
              )}
            </div>
            <div className="stats">
              {[
                [
                  ArrowUp,
                  t("区间上传", "Period upload"),
                  bytes(totals.up),
                  "up",
                ],
                [
                  ArrowDown,
                  t("区间下载", "Period download"),
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
            {totals.uncertain && (
              <div className="notice">
                {t(
                  "此区间含离线后补记流量，发生时间无法精确还原。",
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
                <div className="legend">
                  <span>
                    <i />
                    {t("上传", "Upload")}
                  </span>
                  <span>
                    <i className="blue" />
                    {t("下载", "Download")}
                  </span>
                  <span className="hide-phone">
                    {t("最近 30 分钟", "Last 30 minutes")}
                  </span>
                </div>
              </div>
              {data.speeds.length ? (
                <Plot
                  data={data.speeds}
                  dark={dark}
                  labels={[t("上传", "Upload"), t("下载", "Download")]}
                />
              ) : (
                <div className="empty">
                  {t(
                    "连接下载器后，这里会显示实时曲线",
                    "Connect a downloader to see live transfer rates",
                  )}
                </div>
              )}
            </section>
            <div className="split">
              <section className="card">
                <div className="section-head">
                  <h2>{t("下载器", "Downloaders")}</h2>
                  <button
                    className="icon"
                    aria-label={t("管理下载器", "Manage downloaders")}
                    onClick={() => setPage(3)}
                  >
                    <Plus />
                  </button>
                </div>
                {clients}
              </section>
              <section className="card">
                <div className="section-head">
                  <h2>{t("站点流量统计", "Traffic by site")}</h2>
                  <button
                    className="icon"
                    aria-label={t("管理站点", "Manage sites")}
                    onClick={() => setPage(1)}
                  >
                    <ArrowRight />
                  </button>
                </div>
                {siteTable}
              </section>
            </div>
            <section className="card">
              <div className="section-head">
                <div>
                  <h2>{t("动态带宽分配", "Dynamic bandwidth")}</h2>
                  {guard}
                </div>
                <button onClick={release} disabled={busy}>
                  <ArrowClockwise />
                  {t("恢复系统限速", "Release system limits")}
                </button>
              </div>
              {torrents}
              <p className="hint">
                <Info size={15} />
                {t(
                  "多 Tracker 种子仅统计一次；仅接管你启用的种子，无连接后自动恢复不限速。",
                  "Each torrent is counted once. Only opted-in torrents are controlled; inactive torrents return to unlimited.",
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
              </div>
              {buckets.length ? (
                <Plot
                  data={buckets}
                  bars
                  dark={dark}
                  labels={[t("上传", "Upload"), t("下载", "Download")]}
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
                  "域名匹配忽略 passkey；多个域名可归入一个站点。点击站点编辑。",
                  "Domain rules ignore passkeys. Multiple domains may share a site. Click a site to edit.",
                )}
              </p>
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
        {page === 2 && (
          <>
            <div className="toolbar">
              {guard}
              <span className="grow" />
              <button onClick={release} disabled={busy}>
                <ArrowClockwise />
                {t("恢复系统限速", "Release system limits")}
              </button>
              <button
                className="primary"
                onClick={() =>
                  open("pool", {
                    id: id(),
                    name: "",
                    budget: 10,
                    enabled: false,
                  })
                }
              >
                <Plus />
                {t("添加带宽池", "Add bandwidth pool")}
              </button>
            </div>
            <div className="notice">
              {t(
                "不同出口使用不同带宽池。默认内置恢复保护；整个容器停止期间不能解除限速，重启后恢复。外置守护为可选项，最低额度不代表实际速度。",
                "Use separate uplink pools. Embedded recovery restores limits on restart, but cannot act while the whole container is stopped. An external guardian is optional. Allowances do not guarantee actual speed.",
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
                      open("pool", { ...p, budget: p.budget / MB })
                    }
                  >
                    <GearSix />
                    {t("配置", "Configure")}
                  </button>
                </div>
                <div className="stat-label">
                  {t("总上传额度", "Upload budget")}
                </div>
                <h1>{bytes(p.budget, true)}</h1>
                <p className="subtitle">
                  {c.clients
                    .filter((cl) => cl.pool === p.id)
                    .map((cl) => cl.name)
                    .join(" · ") ||
                    t("尚未分配下载器", "No downloaders assigned")}
                </p>
              </section>
            ))}
            <section className="card">
              <div className="section-head">
                <h2>{t("站点权重与额度", "Site weights & allowances")}</h2>
                <button onClick={() => setPage(1)}>
                  {t("编辑站点", "Edit sites")}
                </button>
              </div>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      {[
                        t("站点", "Site"),
                        t("权重", "Weight"),
                        t("最低额度", "Minimum"),
                        t("最高额度", "Maximum"),
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
                        <td>{s.name}</td>
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
                <h2>{t("按种子接管", "Per-torrent control")}</h2>
                <small>
                  {t("点击种子设置", "Click a torrent to configure")}
                </small>
              </div>
              {torrents}
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
                      }),
                  ],
                  [
                    t("修改登录密码", "Change password"),
                    () => open("password", { current: "", password: "" }),
                  ],
                  [
                    t(
                      "关闭调速并恢复不限速",
                      "Disable control & release limits",
                    ),
                    release,
                  ],
                ].map(([label, fn]) => (
                  <button className="settings-action" key={label} onClick={fn}>
                    {label}
                    <ArrowRight />
                  </button>
                ))}
              </section>
            </div>
            <section className="card">
              <div className="section-head">
                <h2>{t("运行记录", "Activity log")}</h2>
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
        <div
          className="backdrop"
          onClick={(e) =>
            e.target === e.currentTarget && !busy && setModal(null)
          }
        >
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
                      ["", t("仅统计", "Statistics only")],
                      ...c.pools.map((p) => [p.id, p.name]),
                    ])}
                    {toggle("enabled", "启用采集", "Enable monitoring")}
                  </>
                )}
                {modal === "site" && (
                  <>
                    {field("name", "站点名称", "Site name", "text", {
                      required: true,
                    })}
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
                    <div className="field-pair">
                      {field(
                        "weight",
                        "优先权重 1–1000",
                        "Priority weight 1–1000",
                        "number",
                        { min: 1, max: 1000 },
                      )}
                      {field(
                        "minimum",
                        "最低额度 MiB/s",
                        "Minimum MiB/s",
                        "number",
                        { min: 0, step: 0.01 },
                      )}
                    </div>
                    {field(
                      "maximum",
                      "最高额度 MiB/s（0 无上限）",
                      "Maximum MiB/s (0 = uncapped)",
                      "number",
                      { min: 0, step: 0.01 },
                    )}
                    <div className="field-pair">
                      {field(
                        "dayTarget",
                        "日目标 GiB（0 关闭）",
                        "Daily target GiB (0 = off)",
                        "number",
                        { min: 0, step: 0.1 },
                      )}
                      {field(
                        "monthTarget",
                        "月目标 GiB（0 关闭）",
                        "Monthly target GiB (0 = off)",
                        "number",
                        { min: 0, step: 0.1 },
                      )}
                    </div>
                    {field(
                      "afterTarget",
                      "达标后额度 MiB/s（大于 0）",
                      "After-target allowance MiB/s (>0)",
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
                      "总上传额度 MiB/s",
                      "Upload budget MiB/s",
                      "number",
                      { required: true, min: 0.01, step: 0.01 },
                    )}
                    {toggle(
                      "enabled",
                      "启用自动分配（需要恢复保护在线）",
                      "Enable allocation (recovery protection required)",
                    )}
                    <small>
                      {t(
                        "保存后给下载器选择此带宽池，并为种子开启接管。",
                        "Assign downloaders to this pool, then opt torrents into control.",
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
                    {field(
                      "weight",
                      "种子内分配权重",
                      "Torrent allocation weight",
                      "number",
                      { min: 1, max: 1000 },
                    )}
                    {toggle(
                      "manage",
                      "允许系统接管此种子上传限速",
                      "Allow system to manage this torrent upload limit",
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
                    {field(
                      "idleSeconds",
                      "无连接释放延迟，秒（10–300）",
                      "Idle release delay, seconds (10–300)",
                      "number",
                      { min: 10, max: 300 },
                    )}
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
