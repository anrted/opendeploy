package module

import (
	"context"
	"fmt"

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
