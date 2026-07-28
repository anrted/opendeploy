package module

import (
	"context"
	"fmt"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/pkg/contract"
)

func (s *Service) GetDataGridSchema(_ context.Context, moduleID, pageID string) (contract.DataGridSchema, error) {
	current := s.registry.Find(moduleID)
	if current == nil {
		return contract.DataGridSchema{}, fmt.Errorf("module %s not found", moduleID)
	}
	provider, ok := current.(contract.DataGridProvider)
	if !ok {
		return contract.DataGridSchema{}, fmt.Errorf("module does not support datagrid")
	}
	return provider.DataGridSchema(pageID)
}

func (s *Service) GetDataGridData(ctx context.Context, moduleID, pageID string) ([]map[string]any, error) {
	current := s.registry.Find(moduleID)
	if current == nil {
		return nil, fmt.Errorf("module %s not found", moduleID)
	}
	provider, ok := current.(contract.DataGridProvider)
	if !ok {
		return nil, fmt.Errorf("module does not support datagrid")
	}
	return provider.DataGridData(ctx, pageID)
}

func (s *Service) ExecuteDataGridAction(ctx context.Context, moduleID, pageID, actionID string, payload map[string]any) error {
	current := s.registry.Find(moduleID)
	if current == nil {
		return fmt.Errorf("module %s not found", moduleID)
	}
	provider, ok := current.(contract.DataGridProvider)
	if !ok {
		return fmt.Errorf("module does not support datagrid")
	}
	return provider.DataGridAction(ctx, pageID, actionID, payload)
}

func (s *Service) SaveSettings(ctx context.Context, moduleID string, settings map[string]any) error {
	current := s.registry.Find(moduleID)
	if current == nil {
		return fmt.Errorf("module %s not found", moduleID)
	}
	provider, ok := current.(contract.SettingsProvider)
	if !ok {
		return fmt.Errorf("module %s does not support saving settings", moduleID)
	}
	return provider.SaveSettings(ctx, settings)
}

func (s *Service) ProtectionPresets(ctx context.Context, moduleID string) ([]contract.ProtectionPreset, error) {
	current := s.registry.Find(moduleID)
	if current == nil {
		return nil, fmt.Errorf("module %s not found", moduleID)
	}
	provider, ok := current.(contract.ProtectionPresetProvider)
	if !ok {
		return nil, fmt.Errorf("module %s does not support protection presets", moduleID)
	}
	return provider.ProtectionPresets(ctx)
}

func (s *Service) PreviewProtectionPreset(ctx context.Context, moduleID, presetID string, settings map[string]any) (*contract.ProtectionPresetPreview, error) {
	current := s.registry.Find(moduleID)
	if current == nil {
		return nil, fmt.Errorf("module %s not found", moduleID)
	}
	provider, ok := current.(contract.ProtectionPresetProvider)
	if !ok {
		return nil, fmt.Errorf("module %s does not support protection presets", moduleID)
	}
	return provider.PreviewProtectionPreset(ctx, presetID, settings)
}

func (s *Service) SaveProtectionPreset(ctx context.Context, moduleID, presetID string, settings map[string]any, userID, ip string) error {
	current := s.registry.Find(moduleID)
	if current == nil {
		return fmt.Errorf("module %s not found", moduleID)
	}
	provider, ok := current.(contract.ProtectionPresetProvider)
	if !ok {
		return fmt.Errorf("module %s does not support protection presets", moduleID)
	}
	if err := provider.SaveProtectionPreset(ctx, presetID, settings); err != nil {
		s.recordAudit(ctx, userID, "module.preset.save.error", moduleID+":"+presetID, ip, audit.StatusError)
		return err
	}
	s.recordAudit(ctx, userID, "module.preset.save", moduleID+":"+presetID, ip, audit.StatusSuccess)
	return nil
}

func (s *Service) ResetProtectionPreset(ctx context.Context, moduleID, presetID, userID, ip string) error {
	current := s.registry.Find(moduleID)
	if current == nil {
		return fmt.Errorf("module %s not found", moduleID)
	}
	provider, ok := current.(contract.ProtectionPresetProvider)
	if !ok {
		return fmt.Errorf("module %s does not support protection presets", moduleID)
	}
	if err := provider.ResetProtectionPreset(ctx, presetID); err != nil {
		s.recordAudit(ctx, userID, "module.preset.reset.error", moduleID+":"+presetID, ip, audit.StatusError)
		return err
	}
	s.recordAudit(ctx, userID, "module.preset.reset", moduleID+":"+presetID, ip, audit.StatusSuccess)
	return nil
}

func (s *Service) SetProtectionPresetEnabled(ctx context.Context, moduleID, presetID string, enabled bool, userID, ip string) error {
	current := s.registry.Find(moduleID)
	if current == nil {
		return fmt.Errorf("module %s not found", moduleID)
	}
	provider, ok := current.(contract.ProtectionPresetProvider)
	if !ok {
		return fmt.Errorf("module %s does not support protection presets", moduleID)
	}
	if err := provider.SetProtectionPresetEnabled(ctx, presetID, enabled); err != nil {
		s.recordAudit(ctx, userID, "module.preset.toggle.error", moduleID+":"+presetID, ip, audit.StatusError)
		return err
	}
	s.recordAudit(ctx, userID, "module.preset.toggle", moduleID+":"+presetID, ip, audit.StatusSuccess)
	return nil
}

func (s *Service) ReadLog(ctx context.Context, moduleID, logID string, lines int) ([]string, error) {
	current := s.registry.Find(moduleID)
	if current == nil {
		return nil, fmt.Errorf("module %s not found", moduleID)
	}
	provider, ok := current.(contract.LogProvider)
	if !ok {
		return nil, fmt.Errorf("module %s does not support log reading", moduleID)
	}
	return provider.ReadLog(ctx, logID, lines)
}

func (s *Service) ClearLog(ctx context.Context, moduleID, logID string) error {
	current := s.registry.Find(moduleID)
	if current == nil {
		return fmt.Errorf("module %s not found", moduleID)
	}
	provider, ok := current.(contract.LogProvider)
	if !ok {
		return fmt.Errorf("module %s does not support log clearing", moduleID)
	}
	return provider.ClearLog(ctx, logID)
}
