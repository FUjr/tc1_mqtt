package main

import (
	"log"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"
)

type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	TLS      bool   `yaml:"tls"`
}

type Config struct {
	ContainerName       string     `yaml:"container_name"`
	ContainerPasswdPath string     `yaml:"container_passwd_path"`
	HostPasswdPath      string     `yaml:"host_passwd_path"`
	ACLPath             string     `yaml:"acl_path"`
	ACLPatterns         []string   `yaml:"acl_patterns"`
	AdminUsername       string     `yaml:"admin_username"`
	AdminPassword       string     `yaml:"admin_password"`
	RegisterLevel       int        `yaml:"register_level"`
	ContactInfo         string     `yaml:"contact_info"`
	SMTP                SMTPConfig `yaml:"smtp"`
	ListenAddr          string     `yaml:"listen_addr"`
}

var cfg Config

func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, &cfg)
}

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	if err := loadConfig(configPath); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 确保ACL文件存在
	if _, err := os.Stat(cfg.ACLPath); os.IsNotExist(err) {
		if err := os.WriteFile(cfg.ACLPath, []byte(""), 0644); err != nil {
			log.Fatalf("创建ACL文件失败: %v", err)
		}
	}

	mux := http.NewServeMux()

	// API路由
	mux.HandleFunc("/api/register", handleRegister)
	mux.HandleFunc("/api/register/verify", handleVerifyEmail)
	mux.HandleFunc("/api/register/add-mac", handleAddMac)
	mux.HandleFunc("/api/admin/login", handleAdminLogin)
	mux.HandleFunc("/api/admin/users", adminAuth(handleUsers))
	mux.HandleFunc("/api/admin/user/", adminAuth(handleUserOps))
	mux.HandleFunc("/api/admin/acl", adminAuth(handleACL))
	mux.HandleFunc("/api/admin/config", adminAuth(handleConfig))
	mux.HandleFunc("/api/register/status", handleRegisterStatus)

	// 前端静态文件
	fs := http.FileServer(http.Dir("../frontend/dist"))
	mux.Handle("/", fs)

	log.Printf("服务启动在 %s", cfg.ListenAddr)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, mux))
}
