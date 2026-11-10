-- Upgrade the existing model-list configuration to the group model allowlist.
-- Keep saved selections and handle either legacy or renamed column layouts.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'groups'::regclass
          AND attname = 'models_list_config'
          AND NOT attisdropped
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM pg_attribute
            WHERE attrelid = 'groups'::regclass
              AND attname = 'model_allowlist'
              AND NOT attisdropped
        ) THEN
            ALTER TABLE groups RENAME COLUMN models_list_config TO model_allowlist;
        ELSE
            EXECUTE $backfill$
                UPDATE groups
                   SET model_allowlist = models_list_config
                 WHERE COALESCE(model_allowlist, '{}'::jsonb) = '{}'::jsonb
                   AND COALESCE(models_list_config, '{}'::jsonb) <> '{}'::jsonb
            $backfill$;
        END IF;
    END IF;
END
$$;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS model_allowlist JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE groups SET model_allowlist = '{}'::jsonb WHERE model_allowlist IS NULL;

ALTER TABLE groups ALTER COLUMN model_allowlist SET DEFAULT '{}'::jsonb;
ALTER TABLE groups ALTER COLUMN model_allowlist SET NOT NULL;

COMMENT ON COLUMN groups.model_allowlist IS
    'Group model allowlist: constrains both model listing responses and request admission';
