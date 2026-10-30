-- Adopt the existing identity store. Never recreate users or invent migrated history.
DO $$
DECLARE found integer; realm text;
BEGIN
 IF to_regclass('users') IS NULL THEN RAISE EXCEPTION 'identity requires committed legacy bootstrap'; END IF;
 SELECT count(*) INTO found FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='users'
 AND column_name IN ('id','username','display_name','role','password_hash') AND data_type='text' AND is_nullable='NO';
 IF found<>5 THEN RAISE EXCEPTION 'incompatible legacy users columns'; END IF;
 IF NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='users' AND column_name='created_at' AND data_type='timestamp with time zone' AND is_nullable='NO') THEN RAISE EXCEPTION 'incompatible users timestamps'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='users'::regclass AND contype='p' AND pg_get_constraintdef(oid)='PRIMARY KEY (id)') THEN RAISE EXCEPTION 'incompatible users primary key'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='users'::regclass AND contype='u' AND pg_get_constraintdef(oid)='UNIQUE (username)') THEN RAISE EXCEPTION 'incompatible username uniqueness'; END IF;
 IF EXISTS(SELECT 1 FROM users WHERE id !~ '^[0-9a-f]{64}$' OR username !~ '^[a-z][a-z0-9_-]{2,31}$' OR char_length(btrim(display_name))<1 OR role NOT IN ('admin','user') OR password_hash !~ '^[0-9a-f]{32}:[0-9a-f]{64}$') THEN RAISE EXCEPTION 'incompatible legacy identity values'; END IF;
 IF EXISTS(SELECT 1 FROM users GROUP BY lower(btrim(username)) HAVING count(*)>1) THEN RAISE EXCEPTION 'username normalization collision'; END IF;
 FOREACH realm IN ARRAY ARRAY['sessions','drive_sessions'] LOOP
  IF to_regclass(realm) IS NOT NULL THEN
   SELECT count(*) INTO found FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=realm AND column_name IN ('token_hash','user_id','csrf_token') AND data_type='text' AND is_nullable='NO';
   IF found<>3 THEN RAISE EXCEPTION 'incompatible session columns in %',realm; END IF;
   IF NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=realm AND column_name='expires_at' AND data_type='timestamp with time zone' AND is_nullable='NO') THEN RAISE EXCEPTION 'incompatible session expiry in %',realm; END IF;
   IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass(realm) AND contype='p' AND pg_get_constraintdef(oid)='PRIMARY KEY (token_hash)') THEN RAISE EXCEPTION 'incompatible session primary key in %',realm; END IF;
   IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass(realm) AND contype='f' AND confrelid='users'::regclass AND confdeltype='c' AND pg_get_constraintdef(oid) LIKE 'FOREIGN KEY (user_id) REFERENCES users(id)%') THEN RAISE EXCEPTION 'incompatible session identity reference in %',realm; END IF;
  END IF;
 END LOOP;
END $$;
