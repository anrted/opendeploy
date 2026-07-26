package osprovider

import "fmt"

type UbuntuProvider struct{}

func (p *UbuntuProvider) WebUser() string { return "www-data" }
func (p *UbuntuProvider) WebGroup() string { return "www-data" }
func (p *UbuntuProvider) PackageManagerName() string { return "apt" }
func (p *UbuntuProvider) NginxServiceName() string { return "nginx" }
func (p *UbuntuProvider) NginxConfigDir() string { return "/etc/nginx" }
func (p *UbuntuProvider) PHPServiceName(version string) string { return fmt.Sprintf("php%s-fpm", version) }
func (p *UbuntuProvider) DefaultSitePath() string { return "/var/www" }
