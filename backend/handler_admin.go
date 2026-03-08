package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

// 获取所有用户列表
func handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		users, err := getUserInfoList()
		if err != nil {
			jsonError(w, fmt.Sprintf("获取用户列表失败: %v", err), http.StatusInternalServerError)
			return
		}
		jsonResp(w, users)

	case http.MethodPost:
		// 管理员创建用户
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

		if !isValidMAC(req.MAC) {
			jsonError(w, "MAC地址格式不合法", http.StatusBadRequest)
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

		jsonResp(w, map[string]string{"message": "用户创建成功"})

	default:
		jsonError(w, "方法不允许", http.StatusMethodNotAllowed)
	}
}

// 单个用户操作: /api/admin/user/{username}
func handleUserOps(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimPrefix(r.URL.Path, "/api/admin/user/")
	if username == "" {
		jsonError(w, "用户名不能为空", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		// 删除用户
		if !userExists(username) {
			jsonError(w, "用户不存在", http.StatusNotFound)
			return
		}

		// 从passwd中删除用户
		cmd := exec.Command("docker", "exec", cfg.ContainerName,
			"mosquitto_passwd", "-D", cfg.ContainerPasswdPath, username)
		if output, err := cmd.CombinedOutput(); err != nil {
			jsonError(w, fmt.Sprintf("删除用户失败: %s %s", err, string(output)), http.StatusInternalServerError)
			return
		}

		// 删除ACL规则
		if err := removeUserACL(username); err != nil {
			jsonError(w, fmt.Sprintf("删除ACL规则失败: %v", err), http.StatusInternalServerError)
			return
		}

		if err := reloadMosquitto(); err != nil {
			jsonError(w, fmt.Sprintf("重载Mosquitto失败: %v", err), http.StatusInternalServerError)
			return
		}

		jsonResp(w, map[string]string{"message": "用户已删除"})

	case http.MethodPut:
		// 修改用户密码 或 追加MAC地址
		var req struct {
			Password string `json:"password"`
			MAC      string `json:"mac"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "请求格式错误", http.StatusBadRequest)
			return
		}

		if !userExists(username) {
			jsonError(w, "用户不存在", http.StatusNotFound)
			return
		}

		// 追加MAC地址（管理员无需密码验证）
		if req.MAC != "" {
			mac := normalizeMAC(req.MAC)
			if mac == "" {
				jsonError(w, "MAC地址格式不正确", http.StatusBadRequest)
				return
			}
			if macExists(mac) {
				jsonError(w, "该MAC地址已存在", http.StatusConflict)
				return
			}
			if err := appendACLRules(username, mac); err != nil {
				jsonError(w, fmt.Sprintf("追加ACL规则失败: %v", err), http.StatusInternalServerError)
				return
			}
			if err := reloadMosquitto(); err != nil {
				jsonError(w, fmt.Sprintf("重载Mosquitto失败: %v", err), http.StatusInternalServerError)
				return
			}
			jsonResp(w, map[string]string{"message": "MAC地址已追加"})
			return
		}

		if req.Password == "" {
			jsonError(w, "密码不能为空", http.StatusBadRequest)
			return
		}

		if err := createMosquittoUser(username, req.Password); err != nil {
			jsonError(w, fmt.Sprintf("修改密码失败: %v", err), http.StatusInternalServerError)
			return
		}

		if err := reloadMosquitto(); err != nil {
			jsonError(w, fmt.Sprintf("重载Mosquitto失败: %v", err), http.StatusInternalServerError)
			return
		}

		jsonResp(w, map[string]string{"message": "密码已修改"})

	default:
		jsonError(w, "方法不允许", http.StatusMethodNotAllowed)
	}
}

// ACL管理
func handleACL(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		content, err := readACL()
		if err != nil {
			jsonError(w, fmt.Sprintf("读取ACL失败: %v", err), http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"content": content})

	case http.MethodPut:
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "请求格式错误", http.StatusBadRequest)
			return
		}

		if err := writeACL(req.Content); err != nil {
			jsonError(w, fmt.Sprintf("写入ACL失败: %v", err), http.StatusInternalServerError)
			return
		}

		if err := reloadMosquitto(); err != nil {
			jsonError(w, fmt.Sprintf("重载Mosquitto失败: %v", err), http.StatusInternalServerError)
			return
		}

		jsonResp(w, map[string]string{"message": "ACL已更新"})

	default:
		jsonError(w, "方法不允许", http.StatusMethodNotAllowed)
	}
}

// 配置管理（获取/修改运行时配置）
func handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonResp(w, map[string]interface{}{
			"container_name":        cfg.ContainerName,
			"container_passwd_path": cfg.ContainerPasswdPath,
			"host_passwd_path":      cfg.HostPasswdPath,
			"acl_path":              cfg.ACLPath,
			"acl_patterns":          cfg.ACLPatterns,
			"register_level":        cfg.RegisterLevel,
			"contact_info":          cfg.ContactInfo,
		})

	case http.MethodPut:
		var req struct {
			RegisterLevel *int    `json:"register_level"`
			ContactInfo   *string `json:"contact_info"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "请求格式错误", http.StatusBadRequest)
			return
		}

		if req.RegisterLevel != nil {
			if *req.RegisterLevel < 0 || *req.RegisterLevel > 2 {
				jsonError(w, "register_level 只能为 0/1/2", http.StatusBadRequest)
				return
			}
			cfg.RegisterLevel = *req.RegisterLevel
		}
		if req.ContactInfo != nil {
			cfg.ContactInfo = *req.ContactInfo
		}

		jsonResp(w, map[string]string{"message": "配置已更新"})

	default:
		jsonError(w, "方法不允许", http.StatusMethodNotAllowed)
	}
}
