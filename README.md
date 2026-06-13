# AI Monitor (aimon-min)

> 极简 Windows 桌面监控工具 — 实时追踪 DeepSeek API 余额与 Token 用量

[![GitHub](https://img.shields.io/badge/repo-aimon--min-181717?logo=github)](https://github.com/linzivan/aimon-min)

![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)
![Platform](https://img.shields.io/badge/Platform-Windows_10/11-0078D6?logo=windows)
![License](https://img.shields.io/badge/License-MIT-green)

---

## 截图

```
┌─────────────────────┐
│  AI Monitor         │
├─────────────────────┤
│ DeepSeek ¥126.30    │
├─────────────────────┤
│ Today   2.3M Token  │
│ Month   34M Token   │
└─────────────────────┘
```

---

## 功能

### V1 当前功能

- **DeepSeek 余额监控** — 实时查询 API 账户余额
- **账户状态监控** — 检测 API 是否正常响应
- **今日 Token 统计** — 显示当天已用 Token 数
- **本月 Token 统计** — 显示本月累计 Token 数
- **Windows Toast 通知** — 余额不足 / API 异常 / Token 异常增长时弹出系统通知（同类通知 30 分钟内不重复）
- **系统托盘菜单** — 右键托盘图标查看余额、Token 用量，支持手动刷新和退出
- **桌面悬浮 Widget** — 无边框、半透明、始终置顶、可拖动、自动保存位置、不显示任务栏图标、每 60 秒自动刷新

### 后续规划（V2+）

- OpenAI / Claude / Gemini / OpenRouter 支持
- 历史趋势图表
- 自定义刷新间隔
- 通知规则自定义

---

## 快速开始

### 1. 下载

从 [Releases](../../releases) 下载 `AI-Monitor.exe`，或自行编译（见下方）。

### 2. 配置

创建配置文件 `config.yaml`，放在以下任一位置：

- **同目录**：与 `AI-Monitor.exe` 放在同一文件夹
- **AppData**：`%APPDATA%\AI-Monitor\config.yaml`

```yaml
deepseek:
  api_key: "sk-your-api-key-here"
  base_url: "https://api.deepseek.com"

notifications:
  # 余额低于此金额时告警（单位：元）
  balance_threshold: 10.0
  # 当日用量超过月均日用量此百分比时告警
  token_surge_threshold: 50
  # 同类通知最短间隔（分钟）
  cooldown_minutes: 30

monitor:
  # 数据刷新间隔（秒）
  refresh_interval: 60

storage:
  # 数据库路径（留空则自动：%APPDATA%/AI-Monitor/monitor.db）
  db_path: ""
```

### 3. 运行

双击 `AI-Monitor.exe`。

- 托盘区域出现 AI Monitor 图标
- 桌面出现悬浮 Widget 显示实时数据
- 无控制台窗口，静默运行

### 4. 托盘菜单

| 菜单项 | 功能 |
|--------|------|
| DeepSeek ¥xx.xx | 显示当前余额 |
| Today xx Token | 显示今日 Token 用量 |
| Month xx Token | 显示本月 Token 用量 |
| 分隔线 | — |
| Refresh | 立即刷新数据 |
| Settings | 设置（V1 预留） |
| Exit | 退出程序 |

---

## 自行编译

### 前置条件

| 工具 | 版本 | 用途 |
|------|------|------|
| [Go](https://go.dev/dl/) | 1.24+ | 编译 |
| [MinGW-w64](https://www.mingw-w64.org/) | 最新 | CGo 交叉编译 |

> 本项目的体系结构（systray、beeep）依赖 CGo，编译时需要 MinGW-w64 的 `x86_64-w64-mingw32-gcc`。

### 构建步骤

#### 方式一：build.bat（Windows）

```batch
cd D:\ai_code\ai_monitor
scripts\build.bat
```

输出：`AI-Monitor.exe`

#### 方式二：直接编译（命令行）

```batch
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=1
set CC=C:\msys64\mingw64\bin\x86_64-w64-mingw32-gcc.exe
go build -ldflags="-H windowsgui -s -w" -trimpath -o AI-Monitor.exe .
```

> **参数说明**：
> - `-H windowsgui` — 无控制台窗口（GUI 模式）
> - `-s -w` — 去掉 DWARF 调试信息和符号表（减小体积）

---

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                         main.go                             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                  lifecycle.App                           ││
│  │                                                         ││
│  │  ┌──────────────┐   ┌──────────────┐                   ││
│  │  │  Scheduler   │   │   Provider   │                   ││
│  │  │  统一调度中心 │──→│  (Interface) │                   ││
│  │  │  禁止模块自   │   │  ┌─────────┐ │                   ││
│  │  │  建 ticker    │   │  │DeepSeek │ │                   ││
│  │  └──────────────┘   │  └─────────┘ │                   ││
│  │         ↓           └──────┬───────┘                   ││
│  │  ┌──────────────┐         ↓                           ││
│  │  │    Store     │  ┌──────────────┐  ┌──────────────┐ ││
│  │  │   (SQLite)   │  │   Widget     │  │    Tray      │ ││
│  │  │   metrics_   │  │  (Win32原生) │  │  (systray)   │ ││
│  │  │   history    │  │  无边框半透明 │  │  托盘菜单    │ ││
│  │  │   alert_     │  │  始终置顶    │  └──────────────┘ ││
│  │  │   history    │  │  可拖动      │                   ││
│  │  │   system_    │  └──────────────┘                   ││
│  │  │   config     │                                      ││
│  │  └──────────────┘                                      ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

### 核心原则

| 原则 | 说明 |
|------|------|
| **统一调度** | 仅 Scheduler 可创建 Ticker，全部周期性任务必须 `Register()` 注册 |
| **统一生命周期** | 所有组件由 App 统一管理 `Start()` / `Stop()` / `Shutdown()` |
| **Provider 接口** | `type Provider interface { Name(); Collect() }` — 当前仅 `DeepSeekProvider` |
| **YAGNI** | 不为未来扩展引入复杂架构，V1 只做最简单、稳定、可运行的版本 |
| **内存安全** | 防止 Goroutine/ Timer/ SQLite连接/ Channel/ Widget刷新 五种泄漏 |

### 数据流

```
DeepSeek API
    │
    ▼
DeepSeekProvider.Collect()
    │
    ├──→ Store.SaveMetrics()    (SQLite 持久化)
    │
    ├──→ Notifier (阈值检查 + Toast 通知 + 30min 去重)
    │
    └──→ Widget.Update() + Tray.Update() (界面刷新)
```

---

## SQLite 表结构

### metrics_history — 指标历史

```sql
CREATE TABLE metrics_history (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_name   TEXT    NOT NULL,
    balance         REAL    NOT NULL DEFAULT 0,
    currency        TEXT    NOT NULL DEFAULT 'CNY',
    account_status  TEXT    NOT NULL DEFAULT 'active',
    today_tokens    INTEGER NOT NULL DEFAULT 0,
    month_tokens    INTEGER NOT NULL DEFAULT 0,
    collected_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

索引：`collected_at`、`(provider_name, collected_at)`

### alert_history — 告警历史

```sql
CREATE TABLE alert_history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    type        TEXT    NOT NULL,    -- balance_low / api_error / token_surge
    message     TEXT    NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

索引：`(type, created_at)` — 用于去重查询

### system_config — 系统配置

```sql
CREATE TABLE system_config (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

用途：存储 Widget 窗口位置（`widget_pos_x` / `widget_pos_y`）

---

## 资源占用

| 指标 | 目标 | 当前 |
|------|------|------|
| CPU | < 1% | ✅ 空闲时接近 0% |
| 内存 | < 50MB / 目标 30MB | ✅ 预计 < 20MB |
| 磁盘 | — | SQLite 数据量极小 |

---

## 备忘：重构时恢复 Fyne Widget

当前 Widget 使用 Win32 原生 API 实现（`golang.org/x/sys/windows`）。如需切换回 Fyne：

1. 删除 `internal/widget/widget.go`
2. 安装 Fyne：`go get fyne.io/fyne/v2`
3. 用 Fyne 的 `Window` + `SetDecorated(false)` + `canvas.NewText()` 重写
4. 在 `go.mod` 中清理依赖

---

## 许可证

MIT
