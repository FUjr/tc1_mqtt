package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
)

// 支持 D0BAE4618631 / AA:BB:CC:DD:EE:FF / AA-BB-CC-DD-EE-FF 三种格式
var macRegex = regexp.MustCompile(`^([0-9A-Fa-f]{12}|([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2})$`)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type RegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	MAC      string `json:"mac"`
	Code     string `json:"code"` // 邮箱验证码（level=2时必填）
}

func isValidMAC(mac string) bool {
	return macRegex.MatchString(mac)
}

// normalizeMAC 统一转为 AA:BB:CC:DD:EE:FF 格式
func normalizeMAC(mac string) string {
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	if len(mac) == 12 {
		return mac[0:2] + ":" + mac[2:4] + ":" + mac[4:6] + ":" + mac[6:8] + ":" + mac[8:10] + ":" + mac[10:12]
	}
	return mac
}

// handleRegisterStatus 返回注册状态和联系信息
func handleRegisterStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}
	jsonResp(w, map[string]interface{}{
		"register_level": cfg.RegisterLevel,
		"contact_info":   cfg.ContactInfo,
		"mqtt_uri":       cfg.MQTTUri,
		"mqtt_ws_uri":    cfg.MQTTWSUri,
	})
}

// handleVerifyEmail 发送邮箱验证码（level=2 时使用）
// POST /api/register/verify   { "email": "user@example.com" }
func handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}
	if cfg.RegisterLevel != 2 {
		jsonError(w, "当前注册模式不需要邮箱验证", http.StatusBadRequest)
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "请求格式错误", http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if !emailRegex.MatchString(req.Email) {
		jsonError(w, "邮箱格式不正确", http.StatusBadRequest)
		return
	}

	code, err := genCode()
	if err != nil {
		jsonError(w, "验证码生成失败", http.StatusInternalServerError)
		return
	}
	storeCode(req.Email, code)

	if err := sendVerificationEmail(req.Email, code); err != nil {
		jsonError(w, fmt.Sprintf("邮件发送失败: %v", err), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"message": "验证码已发送，请检查邮箱（5分钟内有效）"})
}

// handleRegister 用户注册
func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	if cfg.RegisterLevel == 0 {
		jsonError(w, "注册功能已关闭", http.StatusForbidden)
		return
	}

	var req RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "请求格式错误", http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.MAC = strings.TrimSpace(req.MAC)

	if req.Username == "" || req.Password == "" || req.MAC == "" {
		jsonError(w, "用户名、密码和MAC地址不能为空", http.StatusBadRequest)
		return
	}

	// level=2: 用户名必须是合法邮箱，需验证码
	if cfg.RegisterLevel == 2 {
		req.Username = strings.ToLower(req.Username)
		if !emailRegex.MatchString(req.Username) {
			jsonError(w, "当前注册限制使用邮箱作为用户名", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Code) == "" {
			jsonError(w, "请填写邮箱验证码", http.StatusBadRequest)
			return
		}
		if !verifyCode(req.Username, req.Code) {
			jsonError(w, "验证码错误或已过期", http.StatusBadRequest)
			return
		}
	} else {
		if len(req.Username) < 3 || len(req.Username) > 32 {
			jsonError(w, "用户名长度需在3-32个字符之间", http.StatusBadRequest)
			return
		}
	}

	if len(req.Password) < 6 {
		jsonError(w, "密码长度至少6个字符", http.StatusBadRequest)
		return
	}

	if !isValidMAC(req.MAC) {
		jsonError(w, "MAC地址格式不合法，正确格式如: D0BAE4618631 或 AA:BB:CC:DD:EE:FF", http.StatusBadRequest)
		return
	}

	req.MAC = normalizeMAC(req.MAC)

	if userExists(req.Username) {
		jsonError(w, "用户名已存在", http.StatusConflict)
		return
	}

	if macExists(req.MAC) {
		jsonError(w, "该MAC地址已被注册", http.StatusConflict)
		return
	}

	if err := createMosquittoUser(req.Username, req.Password); err != nil {
		jsonError(w, fmt.Sprintf("创建用户失败: %v", err), http.StatusInternalServerError)
		return
	}

	if err := addACLRules(req.Username, req.MAC); err != nil {
		jsonError(w, fmt.Sprintf("添加ACL规则失败: %v", err), http.StatusInternalServerError)
		return
	}

	if err := reloadMosquitto(); err != nil {
		jsonError(w, fmt.Sprintf("重载Mosquitto失败: %v", err), http.StatusInternalServerError)
		return
	}

	jsonResp(w, map[string]string{"message": "注册成功"})
}

// handleAddMac 为已有用户追加 MAC 地址
// POST /api/register/add-mac  { "username": "...", "password": "...", "mac": "..." }
func handleAddMac(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	if cfg.RegisterLevel == 0 {
		jsonError(w, "注册功能已关闭", http.StatusForbidden)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		MAC      string `json:"mac"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "请求格式错误", http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.MAC = strings.TrimSpace(req.MAC)

	if req.Username == "" || req.Password == "" || req.MAC == "" {
		jsonError(w, "用户名、密码和MAC地址不能为空", http.StatusBadRequest)
		return
	}

	if !isValidMAC(req.MAC) {
		jsonError(w, "MAC地址格式不合法，正确格式如: D0BAE4618631 或 AA:BB:CC:DD:EE:FF", http.StatusBadRequest)
		return
	}

	// 验证用户名+密码（通过 mosquitto_passwd -C 检查）
	if !verifyMosquittoPassword(req.Username, req.Password) {
		jsonError(w, "用户名或密码错误", http.StatusUnauthorized)
		return
	}

	req.MAC = normalizeMAC(req.MAC)

	if macExists(req.MAC) {
		jsonError(w, "该MAC地址已被注册", http.StatusConflict)
		return
	}

	if err := appendACLRules(req.Username, req.MAC); err != nil {
		jsonError(w, fmt.Sprintf("添加ACL规则失败: %v", err), http.StatusInternalServerError)
		return
	}

	if err := reloadMosquitto(); err != nil {
		jsonError(w, fmt.Sprintf("重载Mosquitto失败: %v", err), http.StatusInternalServerError)
		return
	}

	jsonResp(w, map[string]string{"message": "MAC地址已添加"})
}

func createMosquittoUser(username, password string) error {
	cmd := exec.Command("docker", "exec", cfg.ContainerName,
		"mosquitto_passwd", "-b", cfg.ContainerPasswdPath, username, password)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

// verifyMosquittoPassword 通过临时文件验证 mosquitto 密码
func verifyMosquittoPassword(username, password string) bool {
	// mosquitto_passwd 没有直接验证命令，用 -c 创建临时文件对比哈希
	// 最简单可靠的方式：直接调用 mosquitto_sub 尝试连接本地 broker
	// 但这要求 broker 可访问，且密码验证开销大。
	// 此处改用读取 passwd 文件 + bcrypt 验证
	return checkPasswdFile(cfg.HostPasswdPath, username, password)
}

func reloadMosquitto() error {
	cmd := exec.Command("docker", "exec", cfg.ContainerName,
		"kill", "-HUP", "1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}
