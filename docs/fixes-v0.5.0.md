# v0.5.0 可靠性修复与验证

## 修复

- 串口按官方固定命令长度分帧，保留残帧、过滤已知异步帧；焦点请求020A映射0A02。真实上传completion样本为13字节，已作为测试fixture。
- 可取消锁等待、partial writes、读超时设置错误处理、取消时Close，无遗弃reader goroutine。lsof只读检查占用（必须可用）；预检仍有TOCTOU，不能保证外部程序绝不并发。
- 4路电源信息区分UFP ADC、PD ADC、真正PD输入、协商值；有符号/无符号字段正确，移除猜测连接标记；修正0A00偏移、短帧拒绝和最小负数溢出。
- CLI错误JSON单对象非零退出、NoArgs、10..100亮度帮助；screen list显示当前图及槽位占用，明确不是完整CRC列表。
- SHA-256区分图片内容，拒绝CRC冲突及无法验证的legacy同CRC记录。
- 处理后JPEG备份和pending日志在任何淘汰前落盘；commit失败可恢复。新图失败后尽可能恢复旧图，断开后下次上传先恢复。全8槽恢复用模拟测试，不删除用户原有图做实机满槽实验。
- 删除与上传共享OS文件锁，删除确认后同步队列；设备明确notfound也移除失效本地项，但仍返回notfound错误。崩溃自动释放锁。
- 删除输出延后本地保存成功，避免成功JSON与错误JSON混合。
- 删除未用于真实通信的旧fixture Parser和泄漏型WaitResponse，替换为实际低层串口测试。

## 已完成验证

- go fmt / go vet / go test -race / macOS build；Linux amd64 CGO0交叉编译。
- 串口低层fake验证fragmented/multiple/async/terminal、超时、读写错误、取消、Close、并发。
- Mock upload验证journal预写、collision、删除后上传、commit失败重试、满槽删除后新图busy恢复旧图。
- 本机只读info/power/display/list成功；PD真正输入0mV/0mA，PD ADC仅6mV，不再错误混标。
- 专用临时测试图CRC e957：上传 ->相同内容复用 ->切回原图 ->删除 ->再次上传 ->删除。最终原图集仍4张，slots[0,1,2,3]，current=slot3/1253。用户已有图片未删除。

## 剩余边界

硬件Flash不是事务存储，设备拔出期间无法即时回滚；恢复依赖保留的用户私有备份和设备重新连接。CRC16仍是固件的寻址键，SHA仅防止本工具静默内容混淆，不能为固件增加唯一ID。外部App并发修改/同端口换设备不能以CRC证明身份；不要混用。底层驱动无WriteDeadline，Close解除阻塞依赖OS。未知帧严格拒绝，不猜协议。
