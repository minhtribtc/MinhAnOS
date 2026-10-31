-- Files jobs move to the platform job engine (ADR 0010): unfinished and recent jobs are copied into platform_jobs
-- with their IDs, so open Files windows keep polling the same job. files_jobs is emptied and no longer written; it is dropped in
-- a later contract step (migration policy).
INSERT INTO platform_jobs(id,type,actor_id,payload,state,unit,completed,total,attempt,max_attempts,run_after,cancel_requested,lease_until,fencing,error_code,request_id,correlation_id,created_at,updated_at,finished_at)
SELECT id,type,user_id,jsonb_build_object('root',root,'source',source,'destination',destination),state,'bytes',completed,total,attempt,max_attempts,created_at,cancel_requested,lease_until,fencing,error_code,request_id,request_id,created_at,updated_at,finished_at
FROM files_jobs ON CONFLICT(id) DO NOTHING;
-- The rows now live in platform_jobs; the legacy table keeps no personal data behind.
DELETE FROM files_jobs;
