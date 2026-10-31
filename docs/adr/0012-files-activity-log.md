# ADR 0012 — Files activity log levels and live presence

Status: accepted 2026-10-31.

## Context

Files recorded only writes, and in homes and Shared Folders without path or name (app standard row 17), so a
System Admin could not answer "who opened which folder, which files were read or downloaded, who is in Files now".
Companies that run MinhAnOS as their file server need that (access control audits, leaks, mistakes), while a
household may want the opposite. The amount recorded is therefore a choice of the System Admin, not a constant.

## Decision

**Level (files SQL 009, `files_settings` one row).** `minimal` (default; the former behaviour) · `writes` · `access`
· `full`. Changed in central Settings → MinhAnOS Files → *Activity log* (System Admin, `GET|PUT
/api/activity/settings`, If-Match = version, 412 VERSION_CONFLICT). Every change is itself recorded
(`files.activity_level {from,to}`) before it takes effect. The service caches the level for 15 s.

| Level | Recorded |
| --- | --- |
| minimal | Writes (upload, mkdir, rename, move, copy, trash, restore, delete, save text, share/link changes); path and name only in the system root |
| writes | + path and names of every write (`path`, `name`, `destination`, `trashItem`), and writes that failed after being recorded (outcome failure + code) |
| access | + every read: `files.list` (folder opened; first page only), `files.text` (file opened), `files.download`, `files.archive` (items, bytes), `files.search` (query), `files.recent`, `files.trash`; reads and writes refused or failed (outcome denied/failure + code); public-link visitors `files.public_*` (anonymous actor, link ID, owner, link path) |
| full | + client address (last `X-Forwarded-For` hop behind the Desktop proxy), browser (≤200 chars), duration in ms, bytes; and **live presence** |

**Events.** `files_events` gains `outcome success|denied|failure`; the CHECK that forbade `path` outside the system
root is dropped — the rule moved to the code (the level). Writes stay fail-closed: recorded *before* they run
(`AUDIT_UNAVAILABLE` if that fails). Reads are recorded *after* the response, best effort (a failed record is logged,
the user is not blocked), so a database hiccup never makes files unreadable. Retention stays 365 days.

**Live presence (level full, memory only).** `GET /api/activity/live` (System Admin) lists, per person and browser:
the folder being viewed, the file open in the editor, downloads/ZIPs/uploads running with bytes so far and total.
Fed by the server (list/text responses, transfers) and by the Files app, which reports every 30 s and on navigation
(`POST /api/activity/presence`); an entry expires after 75 s without news, at most 5000 entries. It is lost on restart
and never written to the database; the event log stays the authoritative record. Presence is self-reported by the
app, so it shows what is open, not proof of reading.

**Log Center.** Files events show their real outcome and the path as target; new filters *Person* (username) and
*Path / text* (substring of target and details, every source); an *In Files now* panel reads the live list (hidden
for organization admins, who are refused). Events without an actor and `via=link` show as *Visitor (public link)*.

**Transparency.** From `access` on, the Files app shows "Activity in Files is logged" in its sidebar.

## Consequences

- At `access`/`full` the log holds personal paths of homes; only System Admins (`logs.audit.read` without
  organization scope) read them. Organization admins still see events of their organizations' Shared Folders only.
- Event volume grows with `access`/`full` (one row per folder opened). Page-2+ listings are not recorded.
- Further ideas, not built: per-file history in the inspector, per-person timeline, CSV export, alerts on mass delete
  or download, per-level retention, reads by WebDAV/SMB once those exist.
