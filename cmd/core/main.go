package main

import (
	"flag"
	"log"
	"os"

	"github.com/anrted/opendeploy/internal/core/app"
	moduleApache "github.com/anrted/opendeploy/modules/apache"
	moduleCertbot "github.com/anrted/opendeploy/modules/certbot"
	moduleCron "github.com/anrted/opendeploy/modules/cron"
	moduleFail2Ban "github.com/anrted/opendeploy/modules/fail2ban"
	moduleFirewall "github.com/anrted/opendeploy/modules/firewall"
	moduleGit "github.com/anrted/opendeploy/modules/git"
	moduleMySQL "github.com/anrted/opendeploy/modules/mysql"
	moduleNginx "github.com/anrted/opendeploy/modules/nginx"
	moduleNodejs "github.com/anrted/opendeploy/modules/nodejs"
	modulePHP "github.com/anrted/opendeploy/modules/php"
	modulePostgreSQL "github.com/anrted/opendeploy/modules/postgresql"
	"github.com/anrted/opendeploy/pkg/contract"
	"github.com/anrted/opendeploy/pkg/version"

	"go.uber.org/fx"
)

func main() {
	configPath := flag.String("config", "/etc/opendeploy/opendeploy.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		log.Println(version.Info())
		os.Exit(0)
	}

	appFx := fx.New(
		fx.Supply(*configPath),
		app.Module,
		// Provide all built-in modules to the registry
		fx.Provide(
			func() []contract.Module {
				return []contract.Module{
					moduleNginx.New(),
					moduleApache.New(),
					modulePHP.New(),
					moduleNodejs.New(),
					moduleGit.New(),
					moduleCertbot.New(),
					moduleMySQL.New(),
					modulePostgreSQL.New(),
					moduleFirewall.New(),
					moduleFail2Ban.New(),
					moduleCron.New(),
				}
			},
		),
	)

	appFx.Run()
}
