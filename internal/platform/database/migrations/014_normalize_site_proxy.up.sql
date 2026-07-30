UPDATE sites
SET
    proxy_enabled = COALESCE(proxy_enabled, 0),
    proxy_host = COALESCE(NULLIF(TRIM(proxy_host), ''), '127.0.0.1'),
    proxy_port = COALESCE(proxy_port, 0)
WHERE
    proxy_enabled IS NULL
    OR proxy_host IS NULL
    OR TRIM(proxy_host) = ''
    OR proxy_port IS NULL;
