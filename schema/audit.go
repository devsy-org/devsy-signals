// Package schema provides the embedded audit data contract.
package schema

import (
	"embed"
	"fmt"
)

//go:embed audit-allowlist.json
var files embed.FS

// AuditAllowlist returns the bundled audit data contract.
func AuditAllowlist() ([]byte, error) {
	raw, err := files.ReadFile("audit-allowlist.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded audit allow-list: %w", err)
	}
	return raw, nil
}
