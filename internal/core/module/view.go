package module

import (
	"time"

	"github.com/anrted/opendeploy/pkg/contract"
)

type ModuleView struct {
	ID              string                      `json:"id"`
	Name            string                      `json:"name"`
	ModuleVersion   string                      `json:"module_version"`
	SoftwareVersion string                      `json:"software_version,omitempty"`
	Description     string                      `json:"description"`
	Category        string                      `json:"category"`
	Icon            string                      `json:"icon"`
	Dependencies    contract.ModuleDependencies `json:"dependencies"`
	Capabilities    contract.ModuleCapabilities `json:"capabilities"`
	State           State                       `json:"state"`
	InstalledAt     *time.Time                  `json:"installed_at,omitempty"`
	Status          *contract.RuntimeStatus     `json:"status,omitempty"`
	Pages           []contract.ModulePage       `json:"pages,omitempty"`
	Actions         []contract.ActionDef        `json:"actions,omitempty"`
	Logs            []contract.LogDef           `json:"logs,omitempty"`
	SettingsSchema  []contract.SettingField     `json:"settings_schema,omitempty"`
}
