package main

import (
	"bufio"
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// 从passwd文件读取所有用户名
func listUsers() ([]string, error) {
	f, err := os.Open(cfg.HostPasswdPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var users []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) >= 1 && parts[0] != "" {
			users = append(users, parts[0])
		}
	}
	return users, scanner.Err()
}

// 检查用户名是否存在
func userExists(username string) bool {
	users, err := listUsers()
	if err != nil {
		return false
	}
	for _, u := range users {
		if u == username {
			return true
		}
	}
	return false
}

// 从ACL文件解析，检查MAC是否已被使用
func macExists(mac string) bool {
	aclContent, err := os.ReadFile(cfg.ACLPath)
	if err != nil {
		return false
	}

	// 统一去掉分隔符后比较
	clean := strings.ToLower(strings.NewReplacer(":", "", "-", "").Replace(mac))
	content := strings.ToLower(string(aclContent))
	return strings.Contains(content, clean)
}

// 添加ACL规则
// mac 已由 normalizeMAC 标准化为 AA:BB:CC:DD:EE:FF 格式
func addACLRules(username, mac string) error {
	// 4 种 MAC 变体
	macColonUpper := strings.ToUpper(mac)
	macColonLower := strings.ToLower(mac)
	macPlainUpper := strings.ReplaceAll(macColonUpper, ":", "")
	macPlainLower := strings.ReplaceAll(macColonLower, ":", "")

	variants := []string{macColonLower, macColonUpper, macPlainLower, macPlainUpper}

	// 去重（若 pattern 无 %s 则四种结果相同）
	seen := make(map[string]struct{})

	var rules strings.Builder
	rules.WriteString(fmt.Sprintf("\nuser %s\n", username))

	for _, pattern := range cfg.ACLPatterns {
		for _, v := range variants {
			topic := fmt.Sprintf(pattern, v)
			if _, exists := seen[topic]; !exists {
				seen[topic] = struct{}{}
				rules.WriteString(fmt.Sprintf("topic readwrite %s\n", topic))
			}
		}
	}

	f, err := os.OpenFile(cfg.ACLPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(rules.String())
	return err
}

// 读取ACL文件内容
func readACL() (string, error) {
	data, err := os.ReadFile(cfg.ACLPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// 写入ACL文件
func writeACL(content string) error {
	return os.WriteFile(cfg.ACLPath, []byte(content), 0644)
}

// 获取用户的ACL规则
func getUserACL(username string) []string {
	data, err := os.ReadFile(cfg.ACLPath)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	inUser := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == fmt.Sprintf("user %s", username) {
			inUser = true
			result = append(result, line)
			continue
		}
		if inUser {
			if strings.HasPrefix(trimmed, "user ") || trimmed == "" {
				if strings.HasPrefix(trimmed, "user ") {
					break
				}
				continue
			}
			result = append(result, line)
		}
	}
	return result
}

// 删除用户的ACL规则
func removeUserACL(username string) error {
	data, err := os.ReadFile(cfg.ACLPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	inUser := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == fmt.Sprintf("user %s", username) {
			inUser = true
			continue
		}
		if inUser {
			if strings.HasPrefix(trimmed, "user ") {
				inUser = false
				result = append(result, line)
			} else if trimmed == "" {
				// skip empty lines in user block
				continue
			}
			// skip topic lines belonging to this user
			continue
		}
		result = append(result, line)
	}

	return writeACL(strings.Join(result, "\n"))
}

type UserInfo struct {
	Username string   `json:"username"`
	ACLRules []string `json:"acl_rules"`
}

func getUserInfoList() ([]UserInfo, error) {
	users, err := listUsers()
	if err != nil {
		return nil, err
	}

	var result []UserInfo
	for _, u := range users {
		rules := getUserACL(u)
		result = append(result, UserInfo{
			Username: u,
			ACLRules: rules,
		})
	}
	return result, nil
}

// checkPasswdFile 从 mosquitto passwd 文件读取哈希并验证密码
// mosquitto 2.x 默认使用 sha512-pbkdf2，格式: $7$<iterations>$<b64_salt>$<b64_key>
// 注意：mosquitto_passwd 生成的 base64 会换行，同一条记录可能跨多行
func checkPasswdFile(passwdPath, username, password string) bool {
	data, err := os.ReadFile(passwdPath)
	if err != nil {
		return false
	}

	// mosquitto passwd 中只有含 ":" 的行是新条目起始行，
	// 不含 ":" 的行是上一条目的 base64 续行，需拼回去
	lines := strings.Split(string(data), "\n")
	var entries []string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, ":") {
			entries = append(entries, line)
		} else if len(entries) > 0 {
			entries[len(entries)-1] += line
		}
	}

	for _, entry := range entries {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 || parts[0] != username {
			continue
		}
		return verifyMosquittoHash(password, parts[1])
	}
	return false
}

// verifyMosquittoHash 验证 mosquitto passwd 哈希
// 支持 $7$ (sha512-pbkdf2, mosquitto 2.x 默认格式)
// 格式: $7$<iterations>$<base64_salt>$<base64_key>
func verifyMosquittoHash(password, storedHash string) bool {
	if !strings.HasPrefix(storedHash, "$7$") {
		return false
	}
	// 分割 "" / "7" / iterations / salt_b64 / key_b64
	fields := strings.Split(storedHash, "$")
	if len(fields) != 5 {
		return false
	}
	iterations, err := strconv.Atoi(fields[2])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(fields[3])
	if err != nil {
		return false
	}
	storedKey, err := base64.StdEncoding.DecodeString(fields[4])
	if err != nil {
		return false
	}
	dk := pbkdf2.Key([]byte(password), salt, iterations, len(storedKey), sha512.New)
	return bytes.Equal(dk, storedKey)
}

// appendACLRules 为已有用户在 ACL 文件中追加规则（不重复添加 user 行）
func appendACLRules(username, mac string) error {
	macColonUpper := strings.ToUpper(mac)
	macColonLower := strings.ToLower(mac)
	macPlainUpper := strings.ReplaceAll(macColonUpper, ":", "")
	macPlainLower := strings.ReplaceAll(macColonLower, ":", "")
	variants := []string{macColonLower, macColonUpper, macPlainLower, macPlainUpper}

	seen := make(map[string]struct{})
	var rules strings.Builder
	for _, pattern := range cfg.ACLPatterns {
		for _, v := range variants {
			topic := fmt.Sprintf(pattern, v)
			if _, exists := seen[topic]; !exists {
				seen[topic] = struct{}{}
				rules.WriteString(fmt.Sprintf("topic readwrite %s\n", topic))
			}
		}
	}

	// 找到用户块末尾，插入新规则；若用户块不存在则追加
	data, err := os.ReadFile(cfg.ACLPath)
	if err != nil {
		return err
	}

	content := string(data)
	userHeader := fmt.Sprintf("\nuser %s\n", username)
	if strings.Contains(content, userHeader) || strings.Contains(content, fmt.Sprintf("user %s\n", username)) {
		// 追加到文件末尾也可以，mosquitto 按用户块最后出现的规则合并
		f, err := os.OpenFile(cfg.ACLPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = fmt.Fprintf(f, "\nuser %s\n%s", username, rules.String())
		return err
	}

	// 用户块不存在，全新追加
	f, err := os.OpenFile(cfg.ACLPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\nuser %s\n%s", username, rules.String())
	return err
}
