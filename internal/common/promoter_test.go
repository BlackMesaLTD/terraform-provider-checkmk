package common

import (
	"testing"

	"github.com/terraform-provider-checkmk/internal/client"
)

func TestAttributePromoter_APIKey(t *testing.T) {
	versionedTypes := client.NewVersionedTypes("2.4.0p17")
	if versionedTypes == nil {
		t.Fatal("Failed to create VersionedTypes for 2.4.0p17")
	}
	pd := &ProviderData{
		TypeMode: TypeModeAuto,
		Types:    versionedTypes,
		Client:   &client.Client{Version: &client.Version{Major: 2, Minor: 4, Patch: 0, Build: 17}},
	}

	promoter := NewAttributePromoter(pd, "checkmk_host")

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"unprefixed built-in tag group is promoted", "agent", "tag_agent"},
		{"another built-in tag group is promoted", "criticality", "tag_criticality"},
		{"already-prefixed tag group is unchanged", "tag_agent", "tag_agent"},
		{"built-in non-tag field is not promoted", "alias", "alias"},
		{"custom attribute is not promoted", "proxy_port", "proxy_port"},
		{"custom attribute whose tag_ form is unknown is not promoted", "agent_port", "agent_port"},
		{"explicit custom tag is unchanged", "tag_my_custom", "tag_my_custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := promoter.APIKey(tt.key); got != tt.want {
				t.Errorf("APIKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// When type information is unavailable (hollow mode, unknown version, nil
// provider data) the promoter passes every key through unchanged.
func TestAttributePromoter_PassthroughWithoutTypes(t *testing.T) {
	cases := map[string]*ProviderData{
		"nil provider data": nil,
		"nil types (hollow)": {
			TypeMode: TypeModeHollow,
			Types:    nil,
		},
	}

	for name, pd := range cases {
		t.Run(name, func(t *testing.T) {
			promoter := NewAttributePromoter(pd, "checkmk_host")
			for _, key := range []string{"agent", "tag_agent", "alias", "proxy_port"} {
				if got := promoter.APIKey(key); got != key {
					t.Errorf("APIKey(%q) = %q, want %q (passthrough)", key, got, key)
				}
			}
		})
	}
}

// A resource type with no attribute schema (e.g. one that doesn't use a nested
// attributes map) must also pass keys through unchanged.
func TestAttributePromoter_UnknownResourceType(t *testing.T) {
	versionedTypes := client.NewVersionedTypes("2.4.0p17")
	pd := &ProviderData{TypeMode: TypeModeAuto, Types: versionedTypes}

	promoter := NewAttributePromoter(pd, "checkmk_not_a_resource")
	if got := promoter.APIKey("agent"); got != "agent" {
		t.Errorf("APIKey(%q) = %q, want passthrough %q", "agent", got, "agent")
	}
}
