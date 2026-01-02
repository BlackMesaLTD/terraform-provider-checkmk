package common

import (
	"strings"
	"testing"

	"github.com/terraform-provider-checkmk/internal/client"
)

func TestNewFeatures(t *testing.T) {
	tests := []struct {
		name                         string
		version                      *client.Version
		wantUseLabelGroups           bool
		wantHasNotificationAPI       bool
		wantSupportsRuleBulk         bool
		wantSupportsHostLabels       bool
		wantHasPasswordStore         bool
		wantSupportsCustomAttributes bool
	}{
		{
			name:                         "CheckMK 2.4.0p17 - All features enabled",
			version:                      &client.Version{Major: 2, Minor: 4, Patch: 0, Build: 17},
			wantUseLabelGroups:           true,
			wantHasNotificationAPI:       true,
			wantSupportsRuleBulk:         true,
			wantSupportsHostLabels:       true,
			wantHasPasswordStore:         true,
			wantSupportsCustomAttributes: true,
		},
		{
			name:                         "CheckMK 2.3.0p41 - Modern features",
			version:                      &client.Version{Major: 2, Minor: 3, Patch: 0, Build: 41},
			wantUseLabelGroups:           true,
			wantHasNotificationAPI:       true,
			wantSupportsRuleBulk:         true,
			wantSupportsHostLabels:       true,
			wantHasPasswordStore:         true,
			wantSupportsCustomAttributes: true,
		},
		{
			name:                         "CheckMK 2.2.0p43 - Most features",
			version:                      &client.Version{Major: 2, Minor: 2, Patch: 0, Build: 43},
			wantUseLabelGroups:           false,
			wantHasNotificationAPI:       true,
			wantSupportsRuleBulk:         false,
			wantSupportsHostLabels:       true,
			wantHasPasswordStore:         true,
			wantSupportsCustomAttributes: true,
		},
		{
			name:                         "CheckMK 2.2.0p5 - Notification API threshold",
			version:                      &client.Version{Major: 2, Minor: 2, Patch: 0, Build: 5},
			wantUseLabelGroups:           false,
			wantHasNotificationAPI:       true,
			wantSupportsRuleBulk:         false,
			wantSupportsHostLabels:       true,
			wantHasPasswordStore:         true,
			wantSupportsCustomAttributes: true,
		},
		{
			name:                         "CheckMK 2.2.0p4 - Before notification API",
			version:                      &client.Version{Major: 2, Minor: 2, Patch: 0, Build: 4},
			wantUseLabelGroups:           false,
			wantHasNotificationAPI:       false,
			wantSupportsRuleBulk:         false,
			wantSupportsHostLabels:       true,
			wantHasPasswordStore:         true,
			wantSupportsCustomAttributes: true,
		},
		{
			name:                         "CheckMK 2.1.0 - Basic features",
			version:                      &client.Version{Major: 2, Minor: 1, Patch: 0, Build: 0},
			wantUseLabelGroups:           false,
			wantHasNotificationAPI:       false,
			wantSupportsRuleBulk:         false,
			wantSupportsHostLabels:       true,
			wantHasPasswordStore:         false,
			wantSupportsCustomAttributes: true,
		},
		{
			name:                         "CheckMK 2.0.0 - Minimal features",
			version:                      &client.Version{Major: 2, Minor: 0, Patch: 0, Build: 0},
			wantUseLabelGroups:           false,
			wantHasNotificationAPI:       false,
			wantSupportsRuleBulk:         false,
			wantSupportsHostLabels:       false,
			wantHasPasswordStore:         false,
			wantSupportsCustomAttributes: true,
		},
		{
			name:                         "CheckMK 1.6.0 - No modern features",
			version:                      &client.Version{Major: 1, Minor: 6, Patch: 0, Build: 0},
			wantUseLabelGroups:           false,
			wantHasNotificationAPI:       false,
			wantSupportsRuleBulk:         false,
			wantSupportsHostLabels:       false,
			wantHasPasswordStore:         false,
			wantSupportsCustomAttributes: false,
		},
		{
			name:                         "Nil version - Conservative defaults",
			version:                      nil,
			wantUseLabelGroups:           false,
			wantHasNotificationAPI:       false,
			wantSupportsRuleBulk:         false,
			wantSupportsHostLabels:       false,
			wantHasPasswordStore:         false,
			wantSupportsCustomAttributes: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := NewFeatures(tt.version)

			if features.UseLabelGroups != tt.wantUseLabelGroups {
				t.Errorf("UseLabelGroups = %v, want %v", features.UseLabelGroups, tt.wantUseLabelGroups)
			}
			if features.HasNotificationAPI != tt.wantHasNotificationAPI {
				t.Errorf("HasNotificationAPI = %v, want %v", features.HasNotificationAPI, tt.wantHasNotificationAPI)
			}
			if features.SupportsRuleBulk != tt.wantSupportsRuleBulk {
				t.Errorf("SupportsRuleBulk = %v, want %v", features.SupportsRuleBulk, tt.wantSupportsRuleBulk)
			}
			if features.SupportsHostLabels != tt.wantSupportsHostLabels {
				t.Errorf("SupportsHostLabels = %v, want %v", features.SupportsHostLabels, tt.wantSupportsHostLabels)
			}
			if features.HasPasswordStore != tt.wantHasPasswordStore {
				t.Errorf("HasPasswordStore = %v, want %v", features.HasPasswordStore, tt.wantHasPasswordStore)
			}
			if features.SupportsCustomAttributes != tt.wantSupportsCustomAttributes {
				t.Errorf("SupportsCustomAttributes = %v, want %v", features.SupportsCustomAttributes, tt.wantSupportsCustomAttributes)
			}
		})
	}
}

func TestFeatures_String(t *testing.T) {
	// Test with all features enabled
	features := &Features{
		UseLabelGroups:           true,
		HasNotificationAPI:       true,
		SupportsRuleBulk:         true,
		SupportsHostLabels:       true,
		HasPasswordStore:         true,
		SupportsCustomAttributes: true,
	}

	str := features.String()
	if str == "" {
		t.Error("Features.String() returned empty string")
	}

	// Verify it contains all feature names
	expectedParts := []string{
		"LabelGroups=enabled",
		"NotificationAPI=enabled",
		"RuleBulk=enabled",
		"HostLabels=enabled",
		"PasswordStore=enabled",
		"CustomAttributes=enabled",
	}

	for _, part := range expectedParts {
		if !strings.Contains(str, part) {
			t.Errorf("Features.String() missing %q, got: %s", part, str)
		}
	}

	// Test with all features disabled
	features = &Features{
		UseLabelGroups:           false,
		HasNotificationAPI:       false,
		SupportsRuleBulk:         false,
		SupportsHostLabels:       false,
		HasPasswordStore:         false,
		SupportsCustomAttributes: false,
	}

	str = features.String()
	expectedParts = []string{
		"LabelGroups=disabled",
		"NotificationAPI=disabled",
		"RuleBulk=disabled",
		"HostLabels=disabled",
		"PasswordStore=disabled",
		"CustomAttributes=disabled",
	}

	for _, part := range expectedParts {
		if !strings.Contains(str, part) {
			t.Errorf("Features.String() missing %q, got: %s", part, str)
		}
	}
}
