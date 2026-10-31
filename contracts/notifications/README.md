# Notification contracts

Status: producer side implemented for Identity; consumer is Notification Center minimal v1 (`internal/notifycore`, [design](../../docs/notification-log-center.md)). API schemas of the inbox: `index.v1.json`.

- `security-notice.v1.schema.json` describes rows of `identity_notification_outbox` (identity SQL 007). Identity writes
  them in the same transaction as the change: password changed/reset, role changed, sessions revoked, repeated sign-in
  failures (attempts 10 and 20), local recovery. `mandatory` notices must not be mutable by user preferences.
- Details never contain credentials, tokens or usernames; recipients are public user UUIDs.
- Undelivered rows are kept 90 days. The Notification Center worker copies them into its inbox (same id) and sets `deliveredAt` in one transaction.
- `quota-notice.v1.schema.json` describes rows of `storage_notification_outbox` (storage SQL 005). A trigger on the
  quota ledger queues one notice when a scope reaches 80 % (`quota.warning`), 95 % (`quota.critical`) or 100 %
  (`quota.full`) of its limit, in the same transaction as the change; falling below a level re-arms it. A home notifies
  its owner (source `com.minhanos.files`); a Shared Folder or quota folder notifies the active System Admins (source
  `com.minhanos.storage`, with the share name and folder path). Home notices carry no names or paths. Not mandatory.
  Delivered rows are removed after 30 days.
