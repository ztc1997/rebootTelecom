package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// 调试配置从项目根目录的 .env 读取（HOST / USERNAME / PASSWORD）。
func loadEnv(t *testing.T) (host, username, password string) {
	t.Helper()

	data, err := os.ReadFile(".env")
	if err != nil {
		t.Fatalf("read .env failed: %s", err)
	}

	cfg := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		cfg[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	host = cfg["HOST"]
	username = cfg["USERNAME"]
	password = cfg["PASSWORD"]
	if host == "" || username == "" || password == "" {
		t.Fatalf(".env must define HOST, USERNAME and PASSWORD")
	}
	return
}

// loginAndRebootQuery 完成登录、按账户类型选出正确的设备管理页并提取 token。
func loginAndRebootQuery(t *testing.T) (host, rebootQuery string) {
	t.Helper()
	host, username, password := loadEnv(t)
	sk, err := GetSessionKey(getLoginPageUrl(host))
	if err != nil {
		t.Fatal(err)
	}
	body, err := Login(getLoginUrl(host), username, password, sk)
	if err != nil {
		t.Fatal(err)
	}
	page := getDeviceUserPageUrl(host)
	if IsAdminSession(body) {
		page = getDevicePageUrl(host)
	}
	query, err := GetRebootToken(page)
	if err != nil {
		t.Fatal(err)
	}
	return host, query
}

func TestGetSessionKey(t *testing.T) {
	host, _, _ := loadEnv(t)
	sk, err := GetSessionKey(getLoginPageUrl(host))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("sessionKey:", sk)
}

func TestLogin(t *testing.T) {
	host, username, password := loadEnv(t)
	sk, err := GetSessionKey(getLoginPageUrl(host))
	if err != nil {
		t.Fatal(err)
	}
	body, err := Login(getLoginUrl(host), username, password, sk)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("login OK (admin=%v)\n", IsAdminSession(body))
}

// 安全验证：走完前三步（登录 + 提取重启 token），但绝不触发重启。
func TestGetRebootToken(t *testing.T) {
	_, rebootQuery := loginAndRebootQuery(t)
	fmt.Println("reboot token:", rebootQuery)
}

// 真正触发重启的测试，默认跳过，避免误执行导致断网。
// 需要显式设置环境变量 REBOOT_CONFIRM=1 才会运行：
//
//	REBOOT_CONFIRM=1 go test -run TestReboot -v
func TestReboot(t *testing.T) {
	if os.Getenv("REBOOT_CONFIRM") != "1" {
		t.Skip("set REBOOT_CONFIRM=1 to actually reboot the ONU")
	}

	host, rebootQuery := loginAndRebootQuery(t)
	if err := Reboot(getRebootUrl(host, rebootQuery)); err != nil {
		t.Fatal(err)
	}
	fmt.Println("reboot triggered")
}
