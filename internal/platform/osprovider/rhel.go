package osprovider

type RHELProvider struct{}

func (p *RHELProvider) WebUser() string            { return "nginx" } // Or apache, but since we use nginx
func (p *RHELProvider) WebGroup() string           { return "nginx" }
func (p *RHELProvider) PackageManagerName() string { return "dnf" }
func (p *RHELProvider) NginxServiceName() string   { return "nginx" }
func (p *RHELProvider) NginxConfigDir() string     { return "/etc/nginx" }
func (p *RHELProvider) PHPServiceName(version string) string {
	// On RHEL it's usually php-fpm regardless of version if using remi or default
	return "php-fpm"
}
func (p *RHELProvider) DefaultSitePath() string { return "/var/www/html" }
