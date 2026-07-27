package nginx

import "context"

func (m *Module) Install(ctx context.Context) error {
	m.log.InfoContext(ctx, "installing nginx")
	output, err := m.deps.Agent.PackageInstall(ctx, "nginx")
	if err != nil {
		return err
	}
	for line := range output {
		m.log.DebugContext(ctx, "apt-get: "+line)
	}
	return m.deps.Agent.ServiceEnable(ctx, "nginx")
}

func (m *Module) Uninstall(ctx context.Context) error {
	m.log.InfoContext(ctx, "uninstalling nginx")
	_ = m.deps.Agent.ServiceStop(ctx, "nginx")
	output, err := m.deps.Agent.PackageRemove(ctx, "nginx")
	if err != nil {
		return err
	}
	for line := range output {
		m.log.DebugContext(ctx, "apt-get: "+line)
	}
	return nil
}

func (m *Module) Enable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceEnable(ctx, "nginx"); err != nil {
		return err
	}
	return m.deps.Agent.ServiceStart(ctx, "nginx")
}

func (m *Module) Disable(ctx context.Context) error {
	if err := m.deps.Agent.ServiceStop(ctx, "nginx"); err != nil {
		return err
	}
	return m.deps.Agent.ServiceDisable(ctx, "nginx")
}

func (m *Module) Restart(ctx context.Context) error {
	return m.deps.Agent.ServiceRestart(ctx, "nginx")
}
