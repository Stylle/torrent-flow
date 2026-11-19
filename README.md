# 流量台 · Torrent Flow

面向个人 NAS 的 qBittorrent / Transmission 流量统计与上传带宽管理工具。浅色、深色、跟随系统，简体中文 / English，支持手机、平板和桌面。

界面配色与图标风格参考 Uptime Kuma，保留流量统计自身布局；不使用其标识，与该项目无从属关系。

支持通过普通局域网 HTTP 地址添加下载器、站点和带宽池；不依赖仅 HTTPS/localhost 可用的浏览器 UUID API。宽屏主区域在侧栏右侧空间内居中，保留适量两侧留白。

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

默认部署两个容器：主程序负责界面/统计/调度，独立 guardian 只负责主程序掉线后的限速恢复，不是初始化容器。按最新恢复要求，主容器停止期间也需要解除系统限速，因此默认配置 `EXTERNAL_GUARDIAN=true`。二者共享同一个 data 目录，不需要外部数据库、Redis 或额外初始化服务。守护、下载器或主机也不可达时，仍需等待连接恢复。

不要把下载器的 WebUI 暴露到公网。访问地址必须从容器内部可达；Transmission 可填 `http://nas:9091` 或完整 RPC 地址；常见 `/transmission/web/` 地址会自动转换为 `/transmission/rpc`。同主机的 RPC 重定向保留 POST 和会话协商，跨主机/端口跳转会拒绝携带凭据，需明确填写最终 RPC 地址。测试按钮使用当前表单，密码留空则使用已有保存值，不要求先保存。

升级时保留原 data 目录和原部署位置，用当前 `compose.yaml` 更新两个服务，执行 `docker compose pull`、`docker compose up -d`。不要使用 `down -v`。0.1.1/0.1.2 用户更新时会重新加入独立守护；原有双容器用户沿用这两个服务即可。

## 使用顺序

1. 添加 qBittorrent 或 Transmission，先测试当前表单再保存，无须先保存才能测试。
2. 默认提供 **50 Mbps 出口**，即 6.25 MB/s，约 5.96 MiB/s。首次接入默认绑定该出口，请按真实可用上行调整。不同公网出口可另建出口并分别绑定；同一出口的多个下载器共同使用同一份预算。
3. Tracker 域名自动发现，不用录入 passkey；点击站点改名、调权重。多域名别名和 RE2 正则仍可使用。
4. 每个下载器独立“启动自动策略 / 停止自动策略”。默认开始自动分配已有和未来的活跃种子，无需另外逐个开启接管；统计始终采集。停止会清空该下载器所有种子级上传限速，包括此前手工设置的限速，**不会修改下载器全局限速**。离线或失败的清空任务持久化重试。
5. 首页仅保留实时曲线、今日汇总和动态种子列表；种子显示下载器、Tracker 域名、状态和额度，包括待分配或操作失败的活跃种子。下载器列表直接显示绑定出口。

## 分配规则与默认值

| 参数 | 默认 | 作用 |
|---|---|---|
| 站点权重 | 10，范围 1–10 | 降低权重可减少该站上传份额；原有手动权重保留 |
| 每个活跃种子最低额度 | 50 KB/s（50,000 B/s） | 固定最低额度，不因种子多而降低；可在站点配置 |
| 站点最高额度 | 0 | 不额外封顶；若上限低于该站活跃种子的最低额度合计，最低额度优先并告警 |
| 日 / 月目标 | 0 | 默认关闭；开启后达标转低速，不暂停种子，同样不能压低固定最低额度 |
| 采样 / 无连接释放延迟 | 5 秒 / 15 秒 | 无连接且无上传流量持续超时后解除系统施加的限速，防止连接抖动 |
| Webhook / 报告 | 默认关闭 | 配置通道后分别启用日报、周报、月报与告警 |

先按活跃站点权重分配出口总预算，再在站内种子间平分。仅在某个种子的最低额度不足时重新分配。例如预算 1,000 KB/s、A/B/C 权重 1:2:2、活跃种子数 1/1/2，额度为 A 200、B 400、C 的两个各 200 KB/s。

**最低额度优先于出口预算。** 若 1,000 KB/s 出口有 25 个活跃种子且最低为 50 KB/s，将配置合计 1,250 KB/s，并明确警告超预算，不把最低额度压到 40 KB/s。实际链路能否提供这些速度取决于物理带宽和 peer 需求。预算是软件调度参考，不是网络层硬上限；已存在的下载器全局限速也可能压低实际速度。

同一个出口下的 QB/TR 合并计算；不同出口分别计算。两个 512 KB/s 出口的配置预算合计是 1,024 KB/s，各自的种子只参与自身出口分配。详细例子见 [1M 配置说明](docs/UPLOAD-1M.md)。

## 统计与性能

具体日期、周、月、年份在“站点统计”内选择，并显示区间；最多保留一年，前一年的报告可能只覆盖仍保留的数据。总览始终显示今天，不使用隐藏的历史筛选。

种子身份为“下载器 ID＋infohash”，累计计数器差分统计，不用速度乘时间；同一种子多个 Tracker 不重复计量，跨下载器相同 hash 是不同传输。默认按首个可用 Tracker 归属，单种子指定归属优先；元数据缺失显示“Tracker 暂不可用”，不伪造站点。历史未归属流量保留为历史归属，不重写既有数据。删除种子不删除历史；离线补记标记时间不确定。

列表由接口按每页 50 条返回，支持服务端名称/Tracker 搜索；站点总速率仍按完整列表汇总。页面避免重叠轮询，后台标签暂停轮询。qB Tracker 补充查询每轮最多 32 个并缓存，使用主 Tracker 和已有缓存继续展示，避免一次为海量种子逐一查询。确认未变化的限速不重复读取或发送。小时流量合并存储、速度最多每15秒写入、定期清理。

## Webhook、日报与告警

“回调通知”沿用图床的 URL、POST/PUT/PATCH、自定义 JSON 请求头、JSON/Text 请求体模板。默认 JSON 为 `{"title":"{{title}}","content":"{{content}}"}`；支持 `title/content/text/event/time/fields`。请求头加密保存、不回显，留空保留；输入 `{}` 清空已有自定义请求头。“测试当前配置”直接测试草稿，不保存或启用，不要求先保存。

分别启用日报、周报、月报、告警。默认按系统时区 08:00 发送：日报统计前一天，周一统计上一周，每月1日统计上个月；报告包含总上传/下载和站点上传。首次部署不补发很久以前的报告，当前到期任务最多延后24小时。告警覆盖最低额度超预算、下载器离线、限速异常、清空待重试及恢复守护离线，相同告警每15分钟最多一次。

任务持久化，失败最多4次，重试间隔1/5/15分钟；已确认成功不重复发送，停用对应规则后不继续发送。若接收端成功但响应丢失，重试仍可能重复，因此附带 Idempotency-Key，不能保证外部恰好接收一次。“投递记录”和运行记录显示结果。

## 完整备份与恢复

“备份恢复”下载 ZIP：包含账号、下载器凭据、站点/出口/通知配置、统计、日志、通知任务和应用密钥，可导入另一处同项目部署。复用今日想看的 portablebackup 归档校验，恢复前验证归属、文件边界、校验和、数据库及密钥。

先停止所有下载器策略并等待限速清空，再选择备份包、点击导入并确认。损坏或不兼容的备份不会覆盖现有数据；服务器保留恢复前 ZIP。恢复在数据库事务中完成，保留当前安装密钥，将导入凭据重新加密，避免替换独立守护正在打开的数据库文件。需要使用备份账号重新登录；调速暂停，请确认新环境的下载器地址后逐个启动。端口、域名和 Docker 挂载仍由新环境配置。备份包含敏感信息，勿公开分享。

## 停止、断线与恢复

正常退出、闲置释放或主服务掉线由独立守护解除系统施加的限速；主容器异常停止后约30秒触发。用户明确点“停止自动策略”则清空该下载器的所有种子级上传限速，两者范围不同。失败任务不丢弃。下载器、守护或整台主机也离线时必须等重新可达。

限速操作先持久化意图，再写入并读回验证；控制器与恢复命令通过 SQLite 写事务串行化。运行记录包含成功限速、释放、异常及策略开关。用户在下载器外部手工修改被接管的限速时自动控制暂停该种子，重新启动该下载器策略可明确重新接管。

## 数据与安全

SQLite 数据库和 `master.key` 必须一起保留，连接配置通过 AES-GCM 加密保存。登录使用密码哈希、HttpOnly/SameSite Cookie、CSRF 令牌、登录频率限制。应用只供部署者个人使用，没有公开注册。

管理员主动配置的内网下载器需要服务端发起请求；此功能允许访问内网，不能把后台账号交给不可信用户。普通请求不跟随跳转；TR RPC 仅允许有限次数同源跳转（或同主机安全升级到 HTTPS），不会将凭据转发到另一主机或任意端口。建议内网使用或通过 HTTPS 反代，并设置 `COOKIE_SECURE=true`。

可使用上述完整 ZIP 备份；也可先停止所有策略、等待恢复队列清空并停止两个服务，再复制完整 `data/` 目录。不要只复制正在写入的 SQLite 主文件。不要公开分享数据库、密钥、初始密码或连接设置。删除 `master.key` 不能重置密码，只会让连接密文无法解密。

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

MIT。功能调研参考 [torrent-traffic-monitor](https://github.com/ystykxc/torrent-traffic-monitor)，视觉参考 [Uptime Kuma](https://github.com/louislam/uptime-kuma)，本项目独立实现；下载器适配依据 [qBittorrent WebUI API](https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-%28qBittorrent-5.0%29)、[Transmission RPC](https://github.com/transmission/transmission/blob/main/docs/rpc-spec.md)。依赖的 React、Chart.js、Font Awesome、Go x/crypto 与 modernc SQLite 保留各自上游授权。详见依赖清单及 lock 文件。

---

**English:** Self-hosted qBittorrent/Transmission traffic accounting and per-downloader upload allocation. Downloaders share named uplink budgets; site weights range from 1 to 10. Fixed per-torrent minimums take precedence over budgets with a warning. Monitoring stays active when allocation is stopped. Includes paginated lists, portable backups, configurable Webhooks, daily/weekly/monthly reports and persisted delivery retries. The default deployment includes an independent recovery guardian.
