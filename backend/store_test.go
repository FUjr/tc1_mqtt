package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyConfigIncludesAirDeviceACL(t *testing.T) {
	old := cfg
	t.Cleanup(func() { cfg = old })
	for _, operation := range []struct {
		name string
		run  func(string, string) error
	}{
		{"register", addACLRules}, {"append", appendACLRules},
	} {
		t.Run(operation.name, func(t *testing.T) {
			cfg = Config{ACLPath: filepath.Join(t.TempDir(), "acl"), ACLPatterns: []string{"device/ztc1/%s/#", "custom/%s/#", "phicomm/m1/%s/#"}}
			original := "user existing\ntopic readwrite original/#\n"
			if err := os.WriteFile(cfg.ACLPath, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			if err := operation.run("existing", "AA:BB:CC:DD:EE:FF"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(cfg.ACLPath)
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			if !strings.HasPrefix(content, original) {
				t.Fatal("original ACL changed")
			}
			for _, topic := range []string{"phicomm/m1/aabbccddeeff/#", "phicomm/a1/aabbccddeeff/#", "device/ztc1/aabbccddeeff/#", "custom/aabbccddeeff/#"} {
				if strings.Count(content, "topic readwrite "+topic+"\n") != 1 {
					t.Fatalf("missing or duplicate rule: %s", topic)
				}
			}
		})
	}
}
