package main

import "fmt"

// 新光猫（电信天邑定制，ty_httpd 服务）的 8080 管理页面端点。
// 相比旧款华为 HN8145V：
//   - 随机 token 不再由 GetRandCount.asp 提供，而是由登录页 login.html 动态生成 sessionKey
//   - 设备管理页由 e8cdevicemanagement.asp 变为 MD_Device.html
//   - 重启接口由 set.cgi?...ResetBoard 变为 rebootinfo.cgi
func getLoginPageUrl(host string) string {
	return fmt.Sprintf("http://%s:8080/login.html", host)
}

func getLoginUrl(host string) string {
	return fmt.Sprintf("http://%s:8080/login.cgi", host)
}

// getDevicePageUrl 管理员账户的设备管理页。
func getDevicePageUrl(host string) string {
	return fmt.Sprintf("http://%s:8080/MD_Device.html", host)
}

// getDeviceUserPageUrl 非管理员账户(useradmin)的设备管理页。
// 两者重启 token 的参数名不同：管理员为 set6_sessionKey，非管理员为 set4_sessionKey。
func getDeviceUserPageUrl(host string) string {
	return fmt.Sprintf("http://%s:8080/MD_Device_user.html", host)
}

// getRebootUrl 拼接重启地址。query 是从设备管理页提取的完整查询片段，
// 形如 "set6_sessionKey=set6_sessionKey_831"（管理员）或
// "set4_sessionKey=set4_sessionKey_624"（非管理员）。
func getRebootUrl(host, query string) string {
	return fmt.Sprintf("http://%s:8080/rebootinfo.cgi?%s", host, query)
}
