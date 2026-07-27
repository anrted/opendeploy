package site

import (
	"context"
	"fmt"

	"github.com/anrted/opendeploy/internal/core/module"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/pkg/contract"
)

type DeployService struct {
	registry *module.Registry
}

func NewDeployService(registry *module.Registry) *DeployService {
	return &DeployService{registry: registry}
}

func (s *DeployService) Apply(ctx context.Context, moduleID string, action contract.SiteAction, current *Site) error {
	registeredModule := s.registry.Find(moduleID)
	if registeredModule == nil {
		return fmt.Errorf("module not found: %s", moduleID)
	}
	webServer, ok := registeredModule.(contract.WebServerPlugin)
	if !ok {
		return fmt.Errorf("module %s does not implement WebServerPlugin", moduleID)
	}
	return webServer.ApplySite(ctx, action, siteSpec(current))
}

func (s *DeployService) ObtainCertificate(ctx context.Context, domain, rootPath string) error {
	registeredModule := s.registry.Find("certbot")
	if registeredModule == nil {
		return apperrors.Internal("certbot module is not installed", nil)
	}
	certbot, ok := registeredModule.(contract.CertbotPlugin)
	if !ok {
		return apperrors.Internal("certbot module does not implement CertbotPlugin", nil)
	}
	return certbot.ObtainCert(ctx, domain, rootPath)
}

func siteSpec(current *Site) contract.SiteSpec {
	var primaryDomain string
	aliases := make([]string, 0, len(current.Domains))
	for _, domain := range current.Domains {
		if domain.Type == DomainPrimary {
			primaryDomain = domain.Domain
		} else {
			aliases = append(aliases, domain.Domain)
		}
	}
	var appVersion, proxyTarget string
	if current.App.AppVersion != nil {
		appVersion = *current.App.AppVersion
	}
	if current.App.ProxyTarget != nil {
		proxyTarget = *current.App.ProxyTarget
	}
	spec := contract.SiteSpec{
		ID: current.ID, Name: current.Name, PrimaryDomain: primaryDomain,
		Aliases: aliases, RootPath: current.RootPath, AppType: current.App.AppType,
		AppVersion: appVersion, ProxyTarget: proxyTarget,
	}
	if current.SSL != nil {
		spec.SSLEnabled = true
		spec.ForceHTTPS = current.SSL.ForceHTTPS
		if current.SSL.CertPath != nil {
			spec.SSLCert = *current.SSL.CertPath
		}
		if current.SSL.KeyPath != nil {
			spec.SSLKey = *current.SSL.KeyPath
		}
	}
	return spec
}
