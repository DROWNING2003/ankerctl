# v0.4.0 本机 E2E

平台 macOS arm64，A210C CDC 291a:010c；未记录设备序列号。此为单设备实验，不代表所有固件兼容。

## 基线与恢复

- reminder: 300s ->1800s ->300s，每次设备回显一致。
- display timeout: enum1(1m) ->enum2(5m) ->enum1，0216回读一致。
- language: zh ->en ->zh，0216回读一致。
- screensaver: on ->off ->on，0216回读一致。
- focus: enabled true ->false ->true，0A02读取42字节，elapsed保留，未清空统计。
- carousel: off/60s ->10s/on ->状态确认 ->off/60s，原图集不清空。
- current screen: 原slot3/CRC1253；选择1253后回读一致。
- 新增专用测试JPEG上传：CRC e957，6598 bytes；切回1253，再仅删除e957，0A1A00确认。最终 count=4 bitmap=0x0f current slot3/1253，与基线一致。

## 固件差异

0217未返回客户端所述3字节布尔应答，不能作为当前固件屏保开关验证。屏保开关改用0216的byte5，已实测off/on成功。未保留不必要的原始响应内容。

## 单元验证

Control MockTransport：精确TX、reminder/focus解析、timeout/language/switch/select回读、delete状态、参数非法时不打开串口、回显不匹配和解析失败拒绝成功。轮播沿用已有模拟测试。串口分帧/抢占、设备断开等基础设施限制见README。
