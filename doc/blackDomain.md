# 手机激活黑域

- 打开手机的usb调试模式
- 把usb设置为midi类型
- 连接电脑
- 安装adb interface 驱动
- 执行 adb devices
- 在手机上给电脑授权，点击同意
- adb -d shell sh /data/data/me.piebridge.brevent/brevent.sh