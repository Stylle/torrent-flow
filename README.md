# 流量台 · Torrent Flow

面向个人 NAS 的 qBittorrent / Transmission 流量统计与上传带宽管理工具。浅色、深色、跟随系统，简体中文 / English，支持手机、平板和桌面。

![界面预览（演示数据）](docs/screenshots/desktop.png)

## Docker 部署

下载本仓库 `compose.yaml`，在该目录运行：

```sh
docker compose up -d
```

打开 `http://你的主机:28188`，账号为 `admin`。首次启动自动生成密码，只写入数据目录：

```sh
docker exec torrent-flow cat /data/initial-password.txt
```

登录后在设置中修改密码，并删除此初始密码文件。也可以首次启动设置 `ADMIN_PASSWORD`，要求 12–72 字节，包含大小写字母、数字、符号；已存在账户时不会通过环境变量重置。不要把真实密码写进公开 Compose 或 Git。

镜像会准备挂载目录后降权为 UID/GID 10001，普通可写 Docker 绑定目录或 named volume 即可。无需另外创建 data-init 服务。若显式指定 `user:` 或挂载为只读，需要自行授予该用户写入数据目录的权限。

默认 Compose 有两个独立容器：Web/统计控制器与限速恢复守护。它们共享同一个数据目录；不需要 Redis、外部数据库或公网回调。仅需要统计时可只启动 `docker compose up -d torrent-flow`，没有守护时系统拒绝开启自动调速。

不要把下载器的 WebUI 暴露到公网。访问地址必须从容器内部可达；Transmission 填完整 RPC 地址，如 `http://nas:9091/transmission/rpc`。测试按钮使用当前表单，密码留空则使用已有保存值，不要求先保存。

## 使用顺序

1. 下载器 → 添加 qBittorrent 或 Transmission，先测试连接再保存。默认只读采集，不更改下载器。
2. 站点统计 → 添加站点名称、匹配域名（支持多个）及可选 RE2 正则。同站点域名自动合并；域名不依赖 passkey。正则针对完整 Tracker URL，但不会在界面、日志中回显 passkey。
3. 观察统计。首次接入的旧累计值建立基线，不记为今日新增。日/周/月/年按所选时区汇总，保留最近一年。
4. 需要自动调速时，先添加带宽池，给同一出口的下载器选择该池。设置站点权重、最低额度、最高额度，确认守护在线后开启带宽池。
5. 点击种子，选择唯一站点归属并明确开启接管。每个种子只有一个限速，不会因为多个 Tracker 重复计量或同时应用多个策略。

有上传连接的种子参与动态分配。最低额度后按权重分配剩余额度；无连接持续超过释放延迟后恢复种子不限速。实际速度受其他 peer 需求影响，额度不代表实际速度。qBittorrent 使用连接的 leechers 判断需求；Transmission 使用正在接收/感兴趣的 peer，按客户端公开字段作近似判断，不保证严格的网络层公平排队。

日/月上传目标是软目标：达到后转为非零低速额度，不暂停种子。不能同时保证绝对不超目标和保持持续上传。下载器全局限速、排队设置保持不变，站点规则仍由使用者自行确认。

## 断线与恢复

- 正常关闭、关闭调速、解除接管、无连接：只解除系统接管的种子限速，其他个人限速不变。
- 控制器异常停止：独立 guardian 在心跳超过约 30 秒后读回并解除系统限速；恢复失败保留队列重试。
- 网络中断、下载器或整台主机也关机、守护同时停止：无法远程修改下载器，待重新可达后继续恢复。`docker compose down` 同时关闭两个服务，不是独立守护仍在运行的测试。
- 用户在下载器手动改了被接管种子的限速：退出接管、记录外部修改，避免覆盖。
- 页面“恢复系统限速”会关闭带宽池并解除系统限速，不是清空所有下载器限速。

限速命令前持久化旧值与计划值，网络写入后读回验证；独立守护和控制器通过 SQLite 写事务串行化命令。轮询和客户端 API 都有延迟，因此预算是软件调度目标，不是网络层硬上限。

## 统计口径

种子以“下载器实例 ID＋infohash”标识，累计计数器差分计量，不用速率乘时间。多个 Tracker 的种子只有一份统计；跨实例相同 hash 的两次传输分别计量。多个不同站点命中时显示冲突，等待指定主归属。

删除种子不删除历史流量。计数器回退或重新添加建立新基线；不能还原客户端已经丢失的计数。长时间离线后能补记累计增量，但无法精确还原每个字节的时间，相关报表明确标注不确定区间。更改站点规则不重写既有历史，之后增量采用新的归属。

## 数据与安全

SQLite 数据库和 `master.key` 必须一起保留，连接配置通过 AES-GCM 加密保存。登录使用密码哈希、HttpOnly/SameSite Cookie、CSRF 令牌、登录频率限制。应用只供部署者个人使用，没有公开注册。

管理员主动配置的内网下载器需要服务端发起请求；此功能允许访问内网，不能把后台账号交给不可信用户。HTTP 重定向关闭，不会将凭据转发到另一主机。建议内网使用或通过 HTTPS 反代，并设置 `COOKIE_SECURE=true`。

备份前先通过界面关闭调速并确认恢复队列为空，再停止服务，复制完整 `data/` 目录。在其他位置挂载该完整目录即可恢复；不要只复制正在写入的 SQLite 主文件。不要公开分享数据库、密钥、初始密码或连接设置。删除 `master.key` 不能重置密码，只会让连接密文无法解密。

## 开发

Go 1.26+，Node 24+：

```sh
cd frontend
npm ci
npm run build
cd ..
go test ./...
go build -o torrent-flow .
./torrent-flow
```

独立恢复守护：`./torrent-flow --guardian`，共享相同 `DATA_DIR`。默认监听 `:8080`，可通过 `LISTEN_ADDR` 修改。前端演示 `/?demo=1` 为明确标注的示例模式，不访问真实下载器，也不修改真实配置。

## License / 致谢

MIT。功能调研参考 [torrent-traffic-monitor](https://github.com/ystykxc/torrent-traffic-monitor)，本项目独立实现；下载器适配依据 [qBittorrent WebUI API](https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-%28qBittorrent-5.0%29)、[Transmission RPC](https://github.com/transmission/transmission/blob/main/docs/rpc-spec.md)。依赖的 React、Chart.js、Phosphor、Go x/crypto 与 modernc SQLite 保留各自上游授权。详见依赖清单及 lock 文件。

---

**English:** Personal, self-hosted traffic accounting and upload-budget control for multiple qBittorrent / Transmission instances. Counters are measured per torrent, not per tracker; site aliases are grouped without duplicate accounting. Start the Compose stack, read the generated initial password locally, connect downloaders, configure site rules and optionally enable guarded bandwidth allocation. Only opted-in torrents are controlled. An independent guardian releases owned limits after the controller heartbeat expires; recovery cannot occur while the downloader or guardian is unreachable. Keep the entire data directory, including the encryption key, when migrating.
