ALTER TABLE public.users
ADD COLUMN IF NOT EXISTS zitadel_subject text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_zitadel_subject
ON public.users (zitadel_subject)
WHERE zitadel_subject IS NOT NULL;
