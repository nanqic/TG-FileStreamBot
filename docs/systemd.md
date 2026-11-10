# 用 systemd 部署与管理 fsb

本文说明如何把 fsb 作为 systemd Service 运行，以及日常的配置、更新、排障方法。

## 为什么要用 systemd

- **开机自启、崩溃自动拉起**：不用再 `nohup` 手动启动，SSH 断开也不影响。
- **工作目录固定**：`fsb.env`、`fsb.session`、`sessions/`、`logs/` 都按工作目录解析，在其他目录直接执行二进制会报 `ENV file not found: fsb.env` / `required key API_ID missing value`。
- **配合自愈机制**：同一个 bot 连续出现 Telegram 连接错误（如 `CONNECTION_NOT_INITED`、`AUTH_KEY_UNREGISTERED`）达到 `TG_ERROR_EXIT_THRESHOLD` 次（默认 5）时，进程会主动退出，由 systemd 重新拉起，用全新连接恢复服务。**没有 systemd/docker 这类守护进程时，进程退出后就不会再起来。**

> 补充：现在直接执行 `/opt/fsb/fsb-linux-amd64 run` 时，如果当前目录没有 `fsb.env` 而二进制旁边有，程序会自动切换到二进制所在目录。但正式运行仍建议走 systemd。

## 目录约定

```
/opt/fsb/
├── fsb-linux-amd64     # 二进制
├── fsb.env             # 配置（含 BOT_TOKEN 等敏感信息）
├── fsb.session         # 主 bot 会话（自动生成）
├── sessions/           # 多 bot worker 会话（USE_SESSION_FILE=true 时）
└── logs/app.log        # 应用日志（自动轮转：10MB × 3 份，保留 7 天）
```

权限建议：

```bash
cp /home/ubuntu/fsb.service  /etc/systemd/system/
chmod +x /opt/fsb/fsb-linux-amd64
chmod 600 /opt/fsb/fsb.env /opt/fsb/fsb.session
```

## 安装 Service

1. 把仓库里的 [`deploy/fsb.service`](../deploy/fsb.service) 复制到服务器：

   ```bash
   cp fsb.service /etc/systemd/system/fsb.service
   ```

   路径不是 `/opt/fsb` 的话，改 `WorkingDirectory` 和 `ExecStart` 两行。

2. 如果之前是手动 / `nohup` 启动的，先停掉旧进程，避免两个进程抢端口：

   ```bash
   pkill -f 'fsb-linux-amd64 run'
   ```

3. 启用并启动：

   ```bash
   systemctl daemon-reload
   systemctl enable --now fsb
   systemctl status fsb
   ```

## 配置管理

所有配置都在 `/opt/fsb/fsb.env`，模板见 [`fsb.sample.env`](../fsb.sample.env)。修改后重启生效：

```bash
vim /opt/fsb/fsb.env
systemctl restart fsb
```

与本 fork 相关的配置项：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `REDIS_ADDR` | `localhost:6379` | `/v/:filename` 用的 Redis，键形如 `vid:S0003` → message ID |
| `REDIS_PASSWORD` | 空 | Redis 密码 |
| `REDIS_DB` | `0` | Redis 库号 |
| `TG_ERROR_EXIT_THRESHOLD` | `5` | 同一 bot 连续连接错误多少次后退出进程交给 systemd 重启；`0` 关闭 |

建议：

- 改配置前先备份：`cp fsb.env fsb.env.bak.$(date +%F)`。
- `fsb.env` 含 token，**不要提交到 git**（仓库 `.gitignore` 已忽略）。
- 不想把 token 放文件里，也可以用 systemd 的 `Environment=` / `EnvironmentFile=`。环境变量会和 `fsb.env` 合并；同名时以 systemd 设置的为准（加载 `fsb.env` 时不会覆盖已存在的环境变量）。

### 用 drop-in 覆盖，不改主文件

只想调某个参数（比如开启定期重启）时，用 drop-in，升级 service 文件时不会丢：

```bash
systemctl edit fsb
```

在打开的编辑器里写：

```ini
[Service]
# 每 7 天自动重启一次，刷新与 Telegram 的长连接
RuntimeMaxSec=7d
```

保存后执行 `systemctl restart fsb`。查看最终生效的配置：`systemctl cat fsb`。

## 日常操作

| 操作 | 命令 |
| --- | --- |
| 查看状态 | `systemctl status fsb` |
| 启动 / 停止 / 重启 | `systemctl start fsb` / `systemctl stop fsb` / `systemctl restart fsb` |
| 实时日志 | `journalctl -u fsb -f` |
| 最近 200 行 | `journalctl -u fsb -n 200 --no-pager` |
| 看某段时间 | `journalctl -u fsb --since "2026-11-10 06:00" --until "2026-11-10 07:00"` |
| 只看错误 | `journalctl -u fsb -p err` |
| 应用自己的 JSON 日志 | `tail -f /opt/fsb/logs/app.log` |
| 看重启次数 | `systemctl show fsb -p NRestarts` |
| 开机自启开关 | `systemctl enable fsb` / `systemctl disable fsb` |

## 更新二进制

```bash
cd /opt/fsb
cp fsb-linux-amd64 fsb-linux-amd64.bak          # 留一份回滚用
# 上传新的 fsb-linux-amd64 覆盖（scp / rsync 等）
chmod +x fsb-linux-amd64
systemctl restart fsb
journalctl -u fsb -n 50 --no-pager              # 确认出现 "Server started"
```

回滚：

```bash
mv fsb-linux-amd64.bak fsb-linux-amd64 && systemctl restart fsb
```

本地交叉编译 Linux 版本：

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o fsb-linux-amd64 ./cmd/fsb
```

## 排障

| 现象 | 原因 / 处理 |
| --- | --- |
| `ENV file not found: fsb.env` + `required key API_ID missing value` | 工作目录不对。检查 `WorkingDirectory`，或确认 `fsb.env` 在二进制旁边 |
| 页面报 `CONNECTION_NOT_INITED`，HTTP 503 | 与 Telegram 的连接坏了。程序会先换其他 worker 重试，连续失败到阈值后自动退出重启；也可以手动 `systemctl restart fsb` |
| `Too many consecutive Telegram connection errors, exiting` | 自愈机制触发的正常退出，systemd 会在 5 秒后拉起。如果频繁出现，检查服务器网络和 bot token 是否被重置 |
| `systemctl status` 显示 `start-limit-hit` | 10 分钟内重启超过 10 次，通常是配置错误。修好后执行 `systemctl reset-failed fsb && systemctl start fsb` |
| `Video mapping not found in Redis`，HTTP 404 | Redis 里没有 `vid:<编号>` 这个键 |
| `this file was deleted`，HTTP 404 | 群组里对应的消息被删了 |
| 端口被占用 `bind: address already in use` | 还有手动启动的旧进程：`ps -ef \| grep fsb`，停掉后再 `systemctl restart fsb` |

## 可选：外部巡检

自愈机制只能处理"程序还在跑、但 Telegram 请求持续失败"的情况。想再兜一层底，可以加一个 systemd timer，定期请求一个已知能播的视频，非 2xx/3xx 就重启。

`/opt/fsb/healthcheck.sh`：

```bash
#!/bin/sh
# 不跟随重定向：/v/ 正常返回 302，Telegram 出问题返回 5xx
code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "http://127.0.0.1:8080/v/S0003")
case "$code" in
  2*|3*) exit 0 ;;
  *) echo "fsb healthcheck failed: HTTP $code, restarting"; systemctl restart fsb ;;
esac
```

`/etc/systemd/system/fsb-healthcheck.service`：

```ini
[Unit]
Description=fsb health check

[Service]
Type=oneshot
ExecStart=/bin/sh /opt/fsb/healthcheck.sh
```

`/etc/systemd/system/fsb-healthcheck.timer`：

```ini
[Unit]
Description=Run fsb health check every 10 minutes

[Timer]
OnBootSec=5min
OnUnitActiveSec=10min

[Install]
WantedBy=timers.target
```

启用：

```bash
chmod +x /opt/fsb/healthcheck.sh
systemctl daemon-reload
systemctl enable --now fsb-healthcheck.timer
systemctl list-timers | grep fsb
```

注意：巡检用的视频编号要是 Redis 里真实存在、群里也没被删的，否则会误判后反复重启。
