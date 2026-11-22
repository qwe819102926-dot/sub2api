ALTER TABLE groups ADD COLUMN IF NOT EXISTS user_concurrency_limit integer NOT NULL DEFAULT 0 CHECK (user_concurrency_limit >= 0);
