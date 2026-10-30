-- Expand/adopt: never rewrite or delete an existing profile.
CREATE TABLE IF NOT EXISTS user_profiles (
 user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 locale text NOT NULL DEFAULT 'vi-VN' CHECK(locale IN ('vi-VN','en-US')),
 theme text NOT NULL DEFAULT 'light' CHECK(theme IN ('light','dark','system')),
 time_zone text NOT NULL DEFAULT 'Asia/Bangkok'
);
-- Fail closed when adopting a table that does not match the old application contract.
DO $$
DECLARE matched integer;
BEGIN
 SELECT count(*) INTO matched FROM information_schema.columns
 WHERE table_schema=current_schema() AND table_name='user_profiles'
 AND column_name IN ('user_id','locale','theme','time_zone') AND data_type='text' AND is_nullable='NO';
 IF matched<>4 THEN RAISE EXCEPTION 'incompatible existing user_profiles columns'; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='user_profiles'::regclass AND contype='p' AND pg_get_constraintdef(oid)='PRIMARY KEY (user_id)') THEN RAISE EXCEPTION 'missing profile primary key'; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='user_profiles'::regclass AND contype='f' AND confrelid='users'::regclass AND confdeltype='c' AND pg_get_constraintdef(oid) LIKE 'FOREIGN KEY (user_id) REFERENCES users(id)%') THEN RAISE EXCEPTION 'incompatible profile identity reference'; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='user_profiles'::regclass AND contype='c' AND convalidated AND pg_get_constraintdef(oid)='CHECK ((locale = ANY (ARRAY[''vi-VN''::text, ''en-US''::text])))') THEN RAISE EXCEPTION 'incompatible profile locale constraint'; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='user_profiles'::regclass AND contype='c' AND convalidated AND pg_get_constraintdef(oid)='CHECK ((theme = ANY (ARRAY[''light''::text, ''dark''::text, ''system''::text])))') THEN RAISE EXCEPTION 'incompatible profile theme constraint'; END IF;
 IF EXISTS (SELECT 1 FROM user_profiles WHERE locale NOT IN ('vi-VN','en-US') OR theme NOT IN ('light','dark','system')) THEN RAISE EXCEPTION 'unsupported existing profile values'; END IF;
END $$;
