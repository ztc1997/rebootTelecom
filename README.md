# rebootTelecom

## 背景

众所周知，电信装宽带送的光猫一向都特别垃圾，毕竟是消费级的产品。

在没办法更换其硬件的情况下，定时重启成了避免不稳定因素的唯一办法。

该工具主要用在公司里，公司使用了电信定制的华为HN8145V这款型号的光猫。这个机器在公司里每隔两三天就会抽风一次，以至于随机丢包并且非常严重，抽风时完全无法正常用网。为了避免突发疫情远程办公时，无法正常回连办公室的机器，只能写了这个工具定时重启一下光猫。

## 验证环境

* 原设备：电信定制的华为 HN8145V，固件版本号 21HWW41930010。
* 新设备：电信定制的光猫（`ty_httpd` 服务，登录页标识 `SCTY` / `GuangXi`，8080 端口），已适配。

**注意**：两代光猫的 Web 协议完全不同，当前代码针对**新光猫**实现。旧 HN8145V 已不再支持。

### 新旧协议差异（供参考）

| 步骤 | 旧华为 HN8145V | 新光猫（ty_httpd） |
| --- | --- | --- |
| 取 token | `POST /asp/GetRandCount.asp` | `GET /login.html` 中提取 `sessionKey` |
| 登录字段 | `UserName` / `PassWord` / `x.X_HW_Token` | `username` / `password`(base64) / `sessionKey` |
| 会话维持 | Cookie | 无 Cookie，靠源 IP |
| 设备管理页 | `/html/ssmp/devmanage/e8cdevicemanagement.asp` | `/MD_Device.html`（管理员）/ `/MD_Device_user.html`（非管理员） |
| 重启接口 | `POST set.cgi?...ResetBoard` | `GET /rebootinfo.cgi?setN_sessionKey=...` |

### 管理员 / 非管理员账户

两类账户都能重启，但**设备管理页和 token 参数名不同**，程序会根据登录主页里的 `curuser` 自动识别账户类型并选用对应页面：

| 账户 | 设备管理页 | 重启 token 参数 |
| --- | --- | --- |
| `telecomadmin` / `admin`（管理员） | `/MD_Device.html` | `set6_sessionKey=set6_sessionKey_XXX` |
| `useradmin`（非管理员） | `/MD_Device_user.html` | `set4_sessionKey=set4_sessionKey_XXX` |

注意：非管理员账户访问越权的管理员页面会被重定向到登录页，且可能使会话失效，因此必须按账户类型精准选页，不能逐个试探。

## 已知问题

执行重启成功后，由于光猫直接就给重启了，而并没有按照流程把所有进程关闭后再重启。因此，TCP连接会处于卡死状态。

如下：

```bash
# ./rebootTelecom -host "192.168.1.1" -username "***" -password "***" 
panic: dial patch page response failed: Post "http://192.168.1.1:8080/html/ssmp/devmanage/set.cgi?x=InternetGatewayDevice.X_HW_DEBUG.SMP.DM.ResetBoard&RequestFile=html/ssmp/devmanage/e8cdevicemanormal.asp": read tcp *.*.*.*:60386->192.168.1.1:8080: read: connection reset by peer

goroutine 1 [running]:
main.main()
        /data/project/rebootTelecom/main.go:34 +0x605
```

等待机器重启完成之后，重新发出的TCP包会在没有被跟踪的情况下直接被重置，就会报出上边这个错误。

但是！至少看到这个错误时，你可以非常确信重启成功了（笑

## 使用方法

如下：

```bash
# ./rebootTelecom -h
Usage of ./rebootTelecom:
  -dry-run
        Only verify login and reboot token, do NOT actually reboot
  -host string
        The host address without port number, such as '192.168.1.1' (default "localhost")
  -password string
        The password used to login your ONU (default "password")
  -username string
        The username used to login your ONU (default "username")
```

需要三个控制台参数，对应分别是：

* 主机地址。请直接填地址**不要带端口号**，比如：192.168.1.1
* 密码
* 用户名

另外提供一个安全参数 `-dry-run`，只验证登录与重启 token 是否都正常、**不会真正重启**，可用于部署前的连通性测试。

需要注意的是，这个用户名和密码需要是能登录进去半超管界面的。**请打开你的机器的8080端口的页面**，**而不是默认的80端口页面**，尝试使用凭据登录，如果能登录成功，再将凭据填写到此处即可。

执行实例：

```bash
# 真正触发重启
./rebootTelecom -host "主机地址" -username "用户名" -password "密码"

# 仅验证登录和 token，不重启（推荐先跑这个确认适配成功）
./rebootTelecom -host "主机地址" -username "用户名" -password "密码" -dry-run
```

## 定时方式

可以运行在Linux下，通过Crontab定时触发。

但是！**请确保你的Linux系统本身能够通过NTP从互联网同步时间，如果系统时间错误，至于后果你也应该知道了。**

安装方式：

```bash
# 请使用root执行
crontab -e
# 此时会进入crontab任务的编辑页面

# 加入下方内容
0 6 * * * /bin/rebootTelecom -host "主机地址" -username "用户名" -password "密码" > /tmp/rebootTelecom.log
```

**请注意！**

* 下载二进制文件后，请给可执行权限~
* 执行路径请根据实际情况自行修改
* 最后的`>`将控制台输出重定向到了`/tmp/rebootTelecom.log`中，可以去查看输出。每次执行后会被覆盖

