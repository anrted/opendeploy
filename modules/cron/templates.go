package cron

import "github.com/anrted/opendeploy/pkg/contract"

type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Expression  string `json:"expression"`
	User        string `json:"user"`
}

func templates() []Template {
	return []Template{
		{ID: "laravel", Name: "Laravel Scheduler", Command: "php artisan schedule:run", Expression: "* * * * *", User: "www-data"},
		{ID: "wordpress", Name: "WordPress Cron", Command: "php wp-cron.php", Expression: "*/5 * * * *", User: "www-data"},
		{ID: "site-backup", Name: "Site Backup", Command: "/usr/local/bin/backup-site", Expression: "0 2 * * *", User: "root"},
		{ID: "database-backup", Name: "Database Backup", Command: "/usr/local/bin/backup-database", Expression: "30 2 * * *", User: "root"},
		{ID: "log-cleanup", Name: "Log Cleanup", Command: "find /var/log -type f -name '*.log' -mtime +30 -delete", Expression: "0 4 * * 0", User: "root"},
		{ID: "temp-cleanup", Name: "Temporary Files Cleanup", Command: "find /tmp -type f -mtime +7 -delete", Expression: "15 4 * * *", User: "root"},
		{ID: "certbot", Name: "Certbot Renew", Command: "certbot renew --quiet", Expression: "0 3 * * *", User: "root"},
		{ID: "fail2ban", Name: "Fail2Ban Reload", Command: "systemctl reload fail2ban", Expression: "0 5 * * 0", User: "root"},
		{ID: "nginx", Name: "Nginx Reload", Command: "systemctl reload nginx", Expression: "0 5 * * 0", User: "root"},
		{ID: "docker", Name: "Docker Cleanup", Command: "docker system prune -af", Expression: "0 4 * * 0", User: "root"},
		{ID: "git-pull", Name: "Git Pull", Command: "git pull --ff-only", Expression: "*/10 * * * *", User: "deploy"},
		{ID: "composer", Name: "Composer Install", Command: "composer install --no-interaction --no-dev", Expression: "0 3 * * *", User: "deploy"},
		{ID: "npm", Name: "NPM Build", Command: "npm run build", Expression: "0 3 * * *", User: "deploy"},
		{ID: "artisan", Name: "PHP Artisan", Command: "php artisan", Expression: "0 * * * *", User: "www-data"},
		{ID: "custom", Name: "Custom Script", Command: "/path/to/script.sh", Expression: "0 3 * * *", User: "root"},
	}
}

func templateJob(template Template) contract.CronJob {
	return contract.CronJob{ID: template.ID, Name: template.Name, Description: template.Description, Command: template.Command, Expression: template.Expression, User: template.User}
}
