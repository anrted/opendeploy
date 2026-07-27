DROP INDEX IF EXISTS idx_users_active;
DROP INDEX IF EXISTS idx_users_role;
ALTER TABLE users DROP COLUMN is_active;
