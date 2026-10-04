// Package modules embeds the YAML definitions used by every deployment.
package modules

import (
	"embed"
	"fmt"
	"github.com/artpar/apigate/core/schema"
)

//go:embed *.yaml
var definitions embed.FS

func Core() ([]schema.Module, error) {
	names := []string{"plan", "user", "api_key", "upstream", "route", "setting", "entitlement", "plan_entitlement", "webhook", "webhook_delivery", "billing"}
	out := []schema.Module{}
	for _, name := range names {
		b, e := definitions.ReadFile(name + ".yaml")
		if e != nil {
			return nil, e
		}
		m, e := schema.Parse(b)
		if e != nil {
			return nil, fmt.Errorf("%s: %w", name, e)
		}
		out = append(out, m)
	}
	return out, nil
}
