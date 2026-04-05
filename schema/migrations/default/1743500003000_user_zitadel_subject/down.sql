DROP INDEX IF EXISTS idx_users_zitadel_subject;

ALTER TABLE public.users
DROP COLUMN IF EXISTS zitadel_subject;
