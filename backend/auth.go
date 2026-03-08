package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type AdminLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type TokenPayload struct {
	Username string `json:"username"`
	Exp      int64  `json:"exp"`
}

func generateToken(username string) string {
	payload := TokenPayload{
		Username: username,
		Exp:      time.Now().Add(24 * time.Hour).Unix(),
	}
	data, _ := json.Marshal(payload)
	encoded := hex.EncodeToString(data)

	mac := hmac.New(sha256.New, []byte(cfg.AdminPassword))
	mac.Write(data)
	sig := hex.EncodeToString(mac.Sum(nil))

	return encoded + "." + sig
}

func validateToken(token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}

	data, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(cfg.AdminPassword))
	mac.Write(data)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return false
	}

	var payload TokenPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return false
	}
	return time.Now().Unix() < payload.Exp
}

func adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			jsonError(w, "未授权", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if !validateToken(token) {
			jsonError(w, "令牌无效或已过期", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req AdminLoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "请求格式错误", http.StatusBadRequest)
		return
	}

	if req.Username != cfg.AdminUsername || req.Password != cfg.AdminPassword {
		jsonError(w, "用户名或密码错误", http.StatusUnauthorized)
		return
	}

	token := generateToken(req.Username)
	jsonResp(w, map[string]string{"token": token})
}
