-- Rename the former brand while preserving other customized site names.
UPDATE settings
SET value = 'key智API', updated_at = NOW()
WHERE key = 'site_name'
  AND LOWER(BTRIM(value)) IN ('key智中转', 'key 智中转');
