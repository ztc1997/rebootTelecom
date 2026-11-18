package main

import (
	"flag"
	"fmt"
)

func main() {
	var (
		username string
		password string
		host     string
		dryRun   bool
	)

	flag.StringVar(&username, "username", "username", "The username used to login your ONU")
	flag.StringVar(&password, "password", "password", "The password used to login your ONU")
	flag.StringVar(&host, "host", "localhost", "The host address without port number, such as '192.168.1.1'")
	flag.BoolVar(&dryRun, "dry-run", false, "Only verify login and reboot token, do NOT actually reboot")
	flag.Parse()

	// 1. 从登录页获取动态 sessionKey
	sessionKey, err := GetSessionKey(getLoginPageUrl(host))
	if err != nil {
		panic(err)
	}

	// 2. 登录（不返回 Cookie，靠源 IP 维持会话）
	loginBody, err := Login(getLoginUrl(host), username, password, sessionKey)
	if err != nil {
		panic(err)
	}

	// 3. 从设备管理页获取重启 token。
	// 管理员账户用 MD_Device.html(set6_sessionKey)，非管理员用
	// MD_Device_user.html(set4_sessionKey)。必须按账户类型精准选页，
	// 因为非管理员访问越权页面会导致会话失效。
	devicePageUrl := getDeviceUserPageUrl(host)
	if IsAdminSession(loginBody) {
		devicePageUrl = getDevicePageUrl(host)
	}
	rebootQuery, err := GetRebootToken(devicePageUrl)
	if err != nil {
		panic(err)
	}
	fmt.Printf("reboot token acquired: %s\n", rebootQuery)

	if dryRun {
		fmt.Println("dry-run: login OK, reboot token OK. Skip actual reboot.")
		return
	}

	// 4. 触发重启
	if err := Reboot(getRebootUrl(host, rebootQuery)); err != nil {
		panic(err)
	}
	fmt.Println("reboot request sent.")
}
