package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"regexp"
	"strings"
	"time"
)

var (
	// 全局 HTTP 客户端，设置超时避免设备无响应时永久阻塞。
	// 新光猫的 ty_httpd 是老式单连接服务，sessionKey 与 TCP 连接绑定：
	// 若复用同一连接先后请求 login.html 与 login.cgi，登录会失败（404），
	// 因此禁用 keep-alive，让每个请求使用独立连接（与浏览器/curl 行为一致）。
	client = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	// 从 login.html 提取动态 sessionKey，形如：
	//   document.write('<input type="hidden" name="sessionKey" value="'+'489492028'+'"/>')
	sessionKeyReg *regexp.Regexp

	// 从设备管理页的 btnReset() 提取重启 token。
	// 管理员页 MD_Device.html:      rebootinfo.cgi?set6_sessionKey=set6_sessionKey_480
	// 非管理员页 MD_Device_user.html: rebootinfo.cgi?set4_sessionKey=set4_sessionKey_624
	// 两处 token 的参数名不同，需要从页面里连同参数名一起提取，原样拼进重启 URL。
	rebootTokenReg *regexp.Regexp
)

func initRegexp() error {
	var err error
	if sessionKeyReg == nil {
		sessionKeyReg, err = regexp.Compile(`name="sessionKey"[^0-9]*([0-9]+)`)
		if err != nil {
			return fmt.Errorf("init sessionKey regexp failed: %s", err)
		}
	}
	if rebootTokenReg == nil {
		// 锚定到 btnReset() 内 rebootinfo.cgi 之后的 token，避免误匹配到
		// 页面中位于其之前的 restoreClick()/恢复出厂等其他 setN_sessionKey。
		// 捕获两组：参数为(setN_sessionKey)，值为(setN_sessionKey_数字)。
		// 形如：var loc = 'rebootinfo.cgi?'; loc += 'set4_sessionKey=' + 'set4_sessionKey_624';
		rebootTokenReg, err = regexp.Compile(`rebootinfo\.cgi[\s\S]{0,200}?(set[0-9]+_sessionKey)='\s*\+\s*'(set[0-9]+_sessionKey_[0-9]+)`)
		if err != nil {
			return fmt.Errorf("init reboot token regexp failed: %s", err)
		}
	}
	return nil
}

// GetSessionKey 从登录页 login.html 中提取服务端动态生成的 sessionKey。
// 新光猫不再通过 GetRandCount.asp 下发 token，而是把它嵌在登录页里。
func GetSessionKey(url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("get login page failed: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("get login page with wrong status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read login page failed: %s", err)
	}

	if err := initRegexp(); err != nil {
		return "", err
	}

	matches := sessionKeyReg.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return "", fmt.Errorf("sessionKey not found in login page")
	}

	return matches[1], nil
}

// Login 使用用户名、密码(base64)和 sessionKey 登录 8080 管理页面。
// 新光猫登录成功后不返回 Cookie（靠源 IP 维持会话），返回主页面（含 main.cgi）。
// 凭据错误时设备返回 404 并跳转回 login.html。
//
// 注意：光猫的 CGI 对参数顺序敏感，必须按 username -> password -> sessionKey
// 的顺序提交。Go 的 url.Values.Encode() 会按 key 字母序排列，导致登录失败(404)，
// 因此这里手工拼接并按指定顺序编码 body。
//
// 返回登录成功后的主页 HTML，调用方可从中判断账户类型（管理员/非管理员）。
func Login(url, username, password, sessionKey string) (string, error) {
	body := strings.Join([]string{
		"username=" + neturl.QueryEscape(username),
		"password=" + neturl.QueryEscape(base64.StdEncoding.EncodeToString([]byte(password))),
		"sessionKey=" + neturl.QueryEscape(sessionKey),
	}, "&")

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new login request failed: %s", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("post login form failed: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("login failed with status code %d (credential may wrong)", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read login response failed: %s", err)
	}

	if !strings.Contains(string(respBody), "main.cgi") {
		return "", fmt.Errorf("login failed: credential may wrong")
	}

	return string(respBody), nil
}

// IsAdminSession 根据登录主页 HTML 判断当前账户是否为管理员。
// 页面里含形如 var curuser = 'useradmin'; var sysuser = 'admin';
// var telaccount = 'telecomadmin'; 的定义；当 curuser 等于 sysuser 或
// telaccount 时为管理员，否则为非管理员（useradmin）。
func IsAdminSession(loginBody string) bool {
	curuser := extractJsVar(loginBody, "curuser")
	sysuser := extractJsVar(loginBody, "sysuser")
	telaccount := extractJsVar(loginBody, "telaccount")
	if curuser == "" {
		// 无法识别时保守地按非管理员处理（其页面对所有账户更安全）。
		return false
	}
	return curuser == sysuser || curuser == telaccount
}

func extractJsVar(body, name string) string {
	re := regexp.MustCompile(`var\s+` + regexp.QuoteMeta(name) + `\s*=\s*'([^']*)'`)
	m := re.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// GetRebootToken 从设备管理页提取重启按钮使用的 token 查询片段（形如
// "set6_sessionKey=set6_sessionKey_480"）。
//
// 管理员与非管理员账户看到的设备管理页不同：
//   - 管理员:   MD_Device.html      -> set6_sessionKey
//   - 非管理员: MD_Device_user.html -> set4_sessionKey
//
// 注意：必须根据账户类型精准选择页面，不能逐个试探——非管理员账户访问
// 越权的管理员页面会返回跳转脚本，并可能使当前会话失效，影响后续请求。
func GetRebootToken(url string) (string, error) {
	if err := initRegexp(); err != nil {
		return "", err
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("dial device manage page %s failed: %s", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("get device manage page %s with wrong status code: %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body from device manage page failed: %s", err)
	}

	// 访问越权页面时设备返回跳转脚本而非真实页面。
	if strings.Contains(string(body), "parent.location") {
		return "", fmt.Errorf("page %s redirected (no permission)", url)
	}

	matches := rebootTokenReg.FindStringSubmatch(string(body))
	if len(matches) < 3 {
		return "", fmt.Errorf("reboot token not found in page %s", url)
	}

	return matches[1] + "=" + matches[2], nil
}

// Reboot 请求 rebootinfo.cgi 触发光猫重启。
// 光猫收到请求后会立即重启，往往在返回完整响应前就断开 TCP 连接，
// 因此连接被重置/中断属于预期现象，说明重启指令已经送达，视为成功。
func Reboot(url string) error {
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("reboot failed with wrong status code: %d", resp.StatusCode)
	}

	return nil
}
