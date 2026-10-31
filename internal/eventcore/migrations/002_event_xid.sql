-- The watermark of a consumer may only pass an event once no transaction that could still commit a lower sequence
-- number is running (ADR 0010): the inserting transaction ID is compared with the snapshot xmin, which also covers
-- transactions that run longer than any fixed delay.
ALTER TABLE platform_events ADD COLUMN xid xid8 NOT NULL DEFAULT pg_current_xact_id();
