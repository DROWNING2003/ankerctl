# v0.6.0 仅调用已有协议能力

## 下载来源

官方background.formatted.js:99813–99828：0365握手64字节零填充；0265信息64字节零填充；0265数据byte2=1、seq u32BE@3、其余零。103624–103699：0B65 status，0A65信息status@2 sizeu32BE@3 packetsu32BE@7长度11；每包0A65 status@2 seq@3 payload@7，上限57bytes。CLI严格限制图片<=65520、packets=ceil(size/57)、逐包seq及CRC，拒绝busy/错误状态。串口RequestSized仅白名单读取shape，不提供任意原始命令入口。

## 端口来源

官方FormatPortInfoData:98386–98540；USB速度offset8..12；DP 13..17、HDMI18..22；EDID flag@23, DP制造商/产品@24..28、HDMI@29..33，name@34..43/@44..53；flag@3；0A00 UFP@54/56 PD@58/60 negotiated@62/63。0A27物理尺寸来自98369–98384，flag@2 DP u16BE@3/cm@5，HDMI@6/cm@8。不从互相重叠字段猜测DP协议版本。

## 本机只读验证

- ports：DP connected，Mi Monitor，3840×2160 @75Hz，HDMI disconnected，ethernet false；功率独立四通道，0A00功率按官方绝对值展示，与0A39 signed功率语义不同。
- displays：相同DP信息，对角线设备上报27.1英寸；不是物理实测，不替固件背书。
- status：固件、power、ports、physical_display_sizes、display、wallpaper、timer、reminder全部返回data，无error。
- download：当前slot3 CRC1253，导出24822 bytes，JPEG320×240。导出前后当前CRC及slot一致。没有改设置、切换或删除任何壁纸。

## 自动测试

下载合成fixture：多包拼接、CRC、JPEG尺寸、wrong seq、busy、超限；串口分片与已知异步过滤、动态长度白名单、padding验证；端口USBflag/显示器字段/边界及精确TX；status部分失败仅保留该节error。go fmt/vet/test-race/build通过，Linux交叉编译另行验证。
