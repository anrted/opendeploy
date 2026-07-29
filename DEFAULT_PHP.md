# Default PHP Version

`core.default_php` is a creation-time default. When a new site has
`app_type=php` and the request omits `app_version`, SiteService copies this
setting into the site's persisted application version. An explicit version in
the create request always wins.

Changing the setting does not modify existing sites, switch installed PHP
packages, or rewrite existing PHP-FPM pools. Site templates consume the
persisted per-site application version during deployment.

The installer does not force a default because available PHP versions depend
on the configured operating-system repositories.
