# ankerctl

**在终端里控制你的 Anker A210C 扩展坞。**

读取固件版本、查看电压与电流、调节小屏亮度，以及上传自定义壁纸。通过 USB CDC 串口直接与设备通信，无需官方桌面应用或云端 API。

> 非官方、实验性项目，与 Anker 无关联。优先支持 macOS Apple Silicon。协议来自客户端静态分析和有限的真实设备验证，并非 Anker 公开协议。请勿用于需要可靠电气测量的场景。

## 功能

| 功能 | 状态 |
|---|---|
| 自动发现 A210C 串口 | 已实现，macOS 实机验证 |
| 读取 MCU / UI 固件版本 | 已实现，实机验证 |
| 电压、电流与功率快照 | 已实现；方向、连接标记语义仍需验证 |
| 查询小屏设置 | 已实现，当前输出原始响应 |
| 设置亮度（10–100） | 已实现，实机设置后回读验证 |
| 查询当前壁纸槽位与 CRC | 已实现，当前输出原始响应 |
| 上传并激活 JPEG 壁纸 | 已实现，实机验证 |
| JSON 输出与协议调试日志 | 成功路径已实现；错误 JSON 尚待完善 |
| 完整 USB / DP / HDMI 状态 | 计划中 |
| 健康提醒、专注记录及开关 | 已实现，macOS 实机回读验证 |
| 屏幕休眠、语言、屏保开关 | 已实现，实机设置及恢复验证 |
| 已有壁纸切换、单张删除、轮播 | 已实现，实机验证（仅删除专用测试图） |
| 监控、诊断 | 尚未实现，返回 Unsupported |

## 快速开始

### 使用 go install（推荐）

需要 Go 1.22 或更新版本：

```sh
go install github.com/DROWNING2003/ankerctl/cmd/ankerctl@latest
ankerctl --version
ankerctl devices
```

二进制安装到 `go env GOBIN` 指定的目录；未设置时为 `$(go env GOPATH)/bin`（通常是 `$HOME/go/bin`）。若提示找不到命令，将安装目录加入 PATH，例如：

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

更新也使用同一条 `go install ...@latest` 命令。当前实验版本 `v0.4.0` 包含 PNG/JPEG 自动处理、番茄钟、壁纸轮播、上传 FIFO 和版本查询：

```sh
go install github.com/DROWNING2003/ankerctl/cmd/ankerctl@latest
```

如果新版本尚未被当前 Go 代理收录，出现 `no matching versions for query "latest"`，可仅对此次安装绕过代理（不修改全局配置）：

```sh
GOPROXY=direct go install github.com/DROWNING2003/ankerctl/cmd/ankerctl@latest
```

此方式需要 Git 和访问 GitHub 的网络连接。`@latest` 选择最高稳定语义版本标签，更新后请运行 `ankerctl --version` 确认。若仍显示旧版本，使用 `type -a ankerctl` 检查 PATH 中是否存在旧副本。

### 从源码构建

需要 Go 1.22 或更新版本，以及 Git。

```sh
git clone https://github.com/DROWNING2003/ankerctl.git
cd ankerctl
go mod download
go build -o bin/ankerctl ./cmd/ankerctl

./bin/ankerctl devices
./bin/ankerctl info
```

macOS USB 枚举依赖可能需要系统开发工具；构建提示缺少工具时可安装：

```sh
xcode-select --install
```

设备命令默认自动选择唯一匹配的 A210C，不需要指定端口。多设备时手动选择：

```sh
./bin/ankerctl --port /dev/cu.usbmodemXXXX info
```

安装到命令搜索路径（确保 `$HOME/.local/bin` 在 PATH 中）：

```sh
mkdir -p "$HOME/.local/bin"
install -m 755 bin/ankerctl "$HOME/.local/bin/ankerctl"
```

## 使用示例

```sh
ankerctl devices
ankerctl info
ankerctl ports
ankerctl --json ports
ankerctl display get
ankerctl display brightness 50
ankerctl screen list
ankerctl --timeout 30s screen upload ./wallpaper.jpg
```

### 固件版本

实机输出示例：

```text
MCU=v0.0.0.3 UI=ver0.1.0
```

### 供电快照

`ports` 当前显示 UFP 与 PD ADC 通道的电压、电流和计算功率，并非每个 USB 口的独立功耗。

- 保留电流和功率正负号；实际供电方向尚未独立校准。
- 接近零的电压可能是测量底噪。
- 当前连接标记由非零电压推导，不能替代独立连接状态。
- 不可当作经过校准的功率计。

### 小屏与壁纸

`display get` 输出设备响应十六进制；结构化设置展示待完善。

`screen list` **目前仅查询当前壁纸槽位与 CRC，不是完整素材列表**。

输入支持 PNG/JPEG，按真实内容识别，不依赖文件后缀。工具自动等比例缩放到 320×240 范围内，完整保留图片、不裁剪不拉伸，居中并补黑边；透明区域以黑色合成。自动编码为 320×240 JPEG，并逐步降低 JPEG 质量直到不超过 65,520 字节。原图不修改。

输入安全限制：文件最多 32 MiB、图片最多 2000 万像素；超限在打开设备前拒绝。JPEG EXIF 方向暂不自动处理，请提供已摆正的图片。此功能从 v0.2.0 起提供，安装后用 `ankerctl --version` 确认实际执行的版本。

上传包含握手、分块传输、设备完成确认和激活，并回读当前 CRC 后记录成功。

### 壁纸轮播（实验性）

```sh
ankerctl screen carousel status
ankerctl screen carousel interval 60s   # 3..3600 整数秒
ankerctl screen carousel on
ankerctl screen carousel off
```

开启会先开启自定义屏保总开关，再设置轮播。关闭轮播不自动关闭屏保总开关；不清空或同步图集。返回设备应答确认的结果，已在本机验证开启/状态/间隔/关闭并恢复原设置。

### 上传 FIFO（实验性）

上传最多保留 8 槽：未满时新增，满时删除本工具记录中最早上传的图片，再上传新图。重复上传同一处理结果优先激活已有 CRC，不重写 Flash、不改变原上传顺序。CRC16 不是抗碰撞指纹，碰撞风险仍存在。

- 顺序记录存放在系统用户配置目录 `ankerctl/gallery/` 下，按串口路径哈希隔离，不存序列号、原图或文件路径。
- 必须有完整 8 槽上传顺序记录，才会自动淘汰；未知或部分追踪的已满图集会报错，**不猜测槽位年龄、不删除未知图片**。此前版本上传的图片没有顺序记录。
- 自动删除前先激活并回读最早记录的 slot/CRC，匹配后才发送单张删除；验证可能短暂切换当前壁纸。
- 删除成功后才发送新图；若后续上传失败，旧图已删除，无法事务回滚。没有“全部擦除”降级流程。
- 端口路径变化会使用新记录；不得同时用官方 App 或另一实例更改图集。其它软件改图后记录可能失效。
- 上传使用本地独占锁避免本工具两个上传同时执行；崩溃残留锁不会自动抢占，需确认没有上传进程后再人工处理报错中提示的锁文件。

**壁纸写入设备 Flash，请勿高频重复上传，也不要用于视频、动画或秒级仪表盘刷新。**

## 健康提醒与专注记录（v0.4.0）

```sh
ankerctl reminder duration        # 读取健康提醒间隔
ankerctl reminder duration 30m    # 30..3600秒，30秒的整数倍
ankerctl focus status             # 设备专注统计
ankerctl focus on
ankerctl focus off
ankerctl --json focus status
```

专注记录不是录屏或应用使用日志；字段是设备计时、累计时长及最长连续时长。`is_focusing` 沿用官方按 elapsed>0 的推导，可能在 enabled=false 时仍为true，不表示电脑真的正在工作；设备判定算法未知。切换开关可能影响后续统计。健康提醒的展示/提示方式依固件，不承诺声音行为。

## 屏幕设置与素材控制（v0.4.0）

```sh
ankerctl display timeout 5m       # 仅30s/1m/5m/10m/30m/3h；也接受整数秒
ankerctl display language zh      # en/zh/ja/de
ankerctl screen on
ankerctl screen off
ankerctl screen select 1253       # 16进制CRC，不是槽位编号
ankerctl screen delete e957 --yes # 永久删除一张；缺少--yes不会访问设备
```

设置命令均回读检查；屏保开关通过0216回读。当前固件0217实测不是布尔开关应答，CLI不再用它验证。切换CRC不存在则失败；删除status=1返回未找到，不当成功。删除不会修改本地FIFO记录，记录失效时上传保护会拒绝自动淘汰；不要混用外部图集管理工具。删除不可撤销，不提供全部擦除。

## 番茄钟（实验性）

```sh
ankerctl timer duration        # 读取当前时长
ankerctl timer duration 25m    # 设置 25 分钟并检查设备回显
ankerctl timer status          # 原始状态码、剩余秒数、完成序号
ankerctl timer start           # 启动；检查设备接受响应
ankerctl timer stats           # 累计完成次数和秒数
ankerctl --json timer status
```

设置时长支持整数秒或 `25m`、`90s` 等时长格式；必须为 30–3600 秒且为 30 秒的整数倍。设置时长本身不启动计时。

`timer start` 的成功仅表示设备启动响应为接受，建议用 `timer status` 回读。若启动后超时或响应解析失败，设备可能已经开始计时，请先查状态，不要盲目重试启动。状态码保留设备原值，不猜测未知状态。`timer stop` 明确返回 Unsupported；分析的客户端没有固件停止命令。

协议来自真实 CDC 实现（不是客户端 Mock）。本功能已通过构造 fixture 和 MockTransport 测试，尚未进行番茄钟真机启动验证。固件不支持或应答不完整时返回错误，不自动尝试其它命令。

番茄钟从 v0.2.0 起提供；旧版 v0.1.0 不包含此功能。

## 全局参数

| 参数 | 说明 |
|---|---|
| `--port` | 手动指定串口，默认自动发现 |
| `--json` | 成功结果输出 JSON（错误格式尚待完善） |
| `--timeout` | 通信超时，默认 2s；上传建议 30s |
| `--debug` | 原始 TX/RX 十六进制写入 stderr |
| `--help` | 查看帮助 |

调试日志可能包含完整图片数据和设备响应，分享前请检查并脱敏。

## 平台支持

| 平台 | 状态 |
|---|---|
| macOS Apple Silicon | 已进行真实 A210C 测试 |
| macOS Intel | 未验证 |
| Linux | 目标平台，尚未完成构建和实机验证 |
| Windows | 未验证 |

当前实机确认的 CDC 身份为 `291a:010c`，其他变体不保证兼容。

## 安全与已知限制

- 不提供固件升级、Bootloader 或任意原始写入接口。
- 不执行未知命令扫描，不自动停止官方应用，不自动删除壁纸。
- 默认设备输出隐藏序列号。
- **官方应用串口协调与跨进程占用保护尚未完成。使用前请手动退出可能占用设备的程序，不要同时运行多个设备命令。**
- 串口响应目前按单次读取处理，分片或异步应答可能失败；帧聚合、严格超时、取消和断线恢复仍需加强。
- 亮度命令成功仅表示写入发送成功，使用 `display get` 回读验证。
- 上传忙重试、速率限制和完整会话测试尚待完善；一次实机成功不代表所有异常场景均被覆盖。

## 开发

```sh
go fmt ./...
go vet ./...
go test ./...
go build -o bin/ankerctl ./cmd/ankerctl
```

或运行 `make`。当前测试主要覆盖协议构造及解析，并不覆盖全部传输和 CLI 场景。

工程按设备发现、串口传输、协议和命令分层，详见 [协议分析记录](docs/protocol.md)。

欢迎提交 Issue，附上系统和固件版本、复现步骤及脱敏通信样本。请勿提交设备序列号、凭据或完整官方安装包。
