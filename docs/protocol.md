# A210C 协议分析记录

> 状态：部分逆向、实机验证的最小子集，不是公开完整协议。

## 证据等级

- **实机观测**：曾对 A210C 设备成功执行并观察响应。
- **静态逆向**：来自 Anker Electron 应用代码、函数和解包文件。
- **实现推断**：为了隔离代码边界而做的工程判断，不代表固件事实。
- **未知**：当前没有足够证据，工具不得猜测。

## 已确认的 CDC 与屏幕协议

以下对应解包后 `background.js` 的格式化代码位置：

- 设备为 USB CDC，目标实机记录中的 VID `0x291A`；已确认的当前 A210C 在 macOS 枚举为 `/dev/cu.usbmodem113401`，VID `291a`、PID `010c`，USB product string 为 `A210C`。（静态逆向 + 当前机 USB 只读观察）旧 handoff 中的 `0x0C29` 与当前 CDC 枚举 PID 不同，工具以当前实机确认的 `010c` 为识别值。通信参数 115200、8N1。
- `readScreenSettings` 发送 `[02 16]`，等待 `0A 16`，最少 7 字节；响应字段为 `brightness=r[2]`、`lockTime=r[3]`、`language=r[4]`、`screensaverOn=(r[5]==1)`。（`background.formatted.js:102653-102701`）
- `setBrightness` 发送 `[02 12 value]`，应用在发送前把值限制到 10..100。（`background.formatted.js:102703-102710`）
- `setLockScreenTime` 发送 `[02 11 enum]`，应用界面枚举为 `30s=0, 1min=1, 5min=2, 10min=3, 30min=4, 3h=5`；写入后等待约 15ms，无确认响应。（`background.formatted.js:102712-102718`，`index.formatted.js:3147`）
- 自定义屏保开关查询 `[02 17]`，响应 `0A 17 status`；设置 `[02 1F 00|01]`。（`background.formatted.js:102600-102649`、`102728-102745`）
- 上传前要求图片长度 `1..65520` 字节。CRC 为 CRC-16/IBM；分块大小 55。（`background.formatted.js:103040-103090`）
- 上传头为 32 字节：前缀 `02 20 A1 01 FF A2 05 03`；图片长度 offset 8 大端；字段 `A4 03 02` 后写入 `55` 大端；`A5 05 03` 后写入分包数 offset 24；`A6 02` 后写入图片 CRC offset 30。（`background.formatted.js:103060-103090`）
- 上传握手发送 `[02 20 ...]`，等待 `0A 20 status`；status 0 表示允许，status 1 表示 MCU 忙并重试。（`background.formatted.js:103091-103160`）
- 数据包为 `[02 21 seq:u32be payload]`，payload 最多 55 字节；发送后等待 `0A 21`，状态 16 表示完成、17 表示非完成/失败。（`background.formatted.js:103161-103213`）
- 查询当前屏保 `[02 18]`，响应最少 5 字节 `0A 18 slot crc16:u16be`；CRC `0x0000` 被应用解释为无当前屏保。（`background.formatted.js:103233-103276`）
- 激活素材 `[02 19 crc16:u16be]`；官方代码明确记录该命令没有响应包，仅检查写入成功并等待约 20ms。（`background.formatted.js:103278-103306`）
- 删除素材 `[02 1A crc16:u16be]`，响应 `0A 1A status`；0 成功，1 未找到。（`background.formatted.js:103308-103341`）
- 全部擦除 `[02 29]`，响应 `0A 29 status failBitmap`；这属于 Flash 写操作，ankerctl 暂不开放。（`background.formatted.js:103343-103393`）
- 轮播开关 `[02 1B 00|01]`，响应 `0A 1B`；轮播状态查询 `[02 1C]`，响应 `0A 1C enabled count slot bitmap intervalSeconds:u16be`；轮播间隔设置 `[02 1D seconds:u16be]`，应用范围 3..3600 秒。（`background.formatted.js:103395-103527`）
- 固件版本查询前会先发送 `[02 38 02]`，再发送 `[02 30]`；响应 `0A 30` 的字节 2..9 是 MCU ASCII、10..17 是 UI/Flash ASCII。（`background.formatted.js:100419-100457`、`100196-100219`）
- 端口信息轮询 `[02 00]`，解析 `0A 00` 最多 64 字节；屏幕尺寸查询 `[02 27]`，解析 `0A 27` 9 字节；电源快照 `[02 39]`，解析 `0A 39` 16 字节；这些是应用真实查询路径，但当前 Go CLI 尚未实现完整解析。（`background.formatted.js:102908-102987`、`98341-98540`）

## 已实现到 Go 的部分

- 设备命令默认自动枚举唯一 A210C 串口；`--port` 仅用于多设备或人工覆盖。自动发现仍以 VID/PID 和系统枚举为依据，不仅凭 `/dev/cu.*` 名称。
- 真实 E2E 已验证自动发现路径：`info`、`display get`、`screen list`、亮度设置后回读并恢复。


- `internal/protocol/settings.go` 已加入 `0A16` 屏幕设置、`0A30` 固件版本、`0A18` 当前屏保响应解析，以及亮度/锁屏时间请求构造。
- `ankerctl info` 和 `ankerctl display get` 已连接真实串口传输路径；若端口未通过 VID/PID 身份校验或无设备，会返回错误，不伪造数据。
- `ankerctl screen list` 当前发送已确认的 `[02 18]` 查询帧并输出原始响应；真实设备返回已验证为当前 slot/CRC，完整屏保列表语义仍未证实。
- `ankerctl screen upload <file>` 已在真实 A210C 上成功执行：`/tmp/dock-dj.jpg`，14,231 字节，CRC `0xA7FA`；设备返回 `0A 20 00`、`0A 21 10`，随后发送 `02 19 A7 FA`，再次查询得到 `0A 18 01 A7 FA`。这是一次真实写入和激活观测，不代表已实现删除或官方应用并发协调。


`internal/protocol/screen.go` 已实现并测试：

- CRC-16/IBM
- 32 字节屏保上传头
- 55 字节分块包
- `[02 18]` 当前素材查询帧
- `[02 19 crc16]` 激活帧
- `0A 20` / `0A 21` 基本响应状态校验

这些构造器只处理已分析的屏幕协议。CLI 已启用亮度设置和壁纸上传；其它命令是否开放以 README 为准。单元测试输入是人工构造 fixture，除明确标为实机观测的数据外，不代表捕获的设备帧。

## 番茄钟协议（静态确认；未做真机启动验证）

来源：Dock Manager 1.1.7 格式化 `background.js`，真实设备类的 `requestTomatoFrame` / `getTomatoDuration` / `setTomatoDuration` / `getTomatoStatus` / `startTomato` / `getTomatoStats`，行 102402–102499；响应格式来自 `FormatTomatoDurationData` / `FormatTomatoStatusData` / `FormatTomatoStatsData`，行 98592–98609。未采用行 104200 以后的 mock 实现。

| 功能 | 请求 | 响应 |
|---|---|---|
| 时长读取 | `02 2A` | `0A 2A seconds:u32le`（6 字节） |
| 时长设置 | `02 2A seconds:u32le` | 同上，CLI 检查回显与请求一致 |
| 状态 | `02 2B` | `0A 2B state:u8 remaining:u32le completionSeq:u32le`（11 字节） |
| 启动 | `02 2C` | `0A 2C accepted:u8`（3 字节）；客户端以非零表示接受 |
| 统计 | `02 2D` | `0A 2D count:u32le seconds:u32le`（10 字节） |

应用时长限制：整数 30..3600 秒，30 的整数倍。应用等待时间：时长/统计 2s、状态 1.8s、启动 3s；CLI 使用全局 `--timeout`。没有通用帧 CRC 的证据，不附加猜测的校验。停止方法在真实类中直接返回 unsupported，不发送帧。

Go 解析器严格验证响应头和固定长度；当前共享传输层的单次读取限制仍存在，分片或异步混入将报错而不视为成功。测试帧为人工构造 fixture，不是硬件抓包。启动是否切换画面、状态码语义及版本兼容性仍未知。

## 分析制品来源

Anker Dock Manager 1.1.7 macOS 安装镜像 SHA-256：`93c7fc07d222bbc67472e815424b1de9125cd8e550b5285f7513cb87679281fc`。
ASAR SHA-256：`46627b543dd4ee518d9dec6880a9911c072d1c20ea650e18e6df36ba7e859a25`。
上述格式化行号仅供本次分析定位；仓库不分发官方 JS 或原生模块。REA inventory 记录了 7 个原生 unpacked 模块与 ASAR integrity metadata 的哈希不一致，原因未确认；已作为 observed-untrusted 保留，不作为完整性证明。JS 静态分析图因资源限制覆盖不完整，不能用于证明不存在其他协议行为。

## 仍未知或暂不开放

完整握手/仲裁、所有状态字段的稳定版本兼容性、设备信息字段、串口占用协调 WebSocket 的完整协议、错误码、不同固件的差异、断线恢复、Flash 原子性、最大可靠速率、完整屏保读取语义仍需更多实机样本。

`ws://127.0.0.1:47291` 是官方应用串口协调通道，不是设备控制 API；当前不 fuzz，也不发送未知控制帧。

## 安全边界

默认只读；未知命令返回 `Unsupported`。不自动停止官方应用，不抢占被占用串口，不实现 MCU/Flash 固件升级、全部擦除或任意原始写入。屏保删除必须在未来 CLI 中显式确认。日志不保留设备序列号、凭据、cookies 或个人内容。
