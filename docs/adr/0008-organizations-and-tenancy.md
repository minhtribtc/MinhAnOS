# ADR 0008 — Organizations, tenancy and storage allocations

Status: accepted (2026-10-31). Builds on ADR 0005 (identity policy), ADR 0006 (delegation) and the storage
model ([storage-model-review](../storage-model-review.md)). One codebase serves a single NAS (one organization)
and a hosted cloud (many organizations); there is no billing.

## Decision

**Identity owns tenancy.** Organizations (`identity_organizations`) form a tree, at most 8 levels deep. The
parent is fixed at creation (no re-parenting in v1), so the tree cannot form a cycle. `path` holds the IDs from
the top-level organization down to the organization itself and is set by a trigger. An organization is
*effectively active* only while it and every ancestor are `active`.

**Identity, membership, role and group are separate.** An account (`users`) exists once, platform-wide. It joins
organizations through `identity_memberships(organization_id, user_id, role admin|member)`. Platform RBAC (System
Admin, user-admin, custom roles; ADR 0005/0006) stays for platform work. Organization authority is
relationship-based and computed by `identity_org_role(user, org)`:

| Role | Meaning |
|---|---|
| `root` | System Admin (`users.role='admin'`). Manages pools, the homes share, platform settings, top-level organizations and their allocations; may manage any organization |
| `admin` | Direct admin of the organization. Manages its members, groups, Shared Folders, folder quotas and permissions |
| `manager` | Admin of an ancestor with `allow_children`. Manages the descendant's *structure* (name, status, children, members and admins) and allocations, not its data |
| `member` | May use what the organization's permissions grant |

Ancestor admins and Root do **not** see an organization's data. Opening a Shared Folder needs membership of the
owning, effectively active organization *and* an explicit permission entry (deny wins). Groups may belong to an
organization (`identity_groups.organization_id`). Only members can be put in an organization's group, and leaving
the organization removes the account from its groups and from its shares' permission lists.

**Tenant context is server-derived, per request.** No session-level "active organization" is stored. A client
names an organization only as a *selector* in the path (`/organizations/{id}/…`) or a query filter. The server
authorizes `identity_org_role` on every request, and resources found by ID (a share, folder or allocation) are
authorized through their owning organization. A data request is never authorized by a client-sent organization.
Files lists the Shared Folders of every organization the user belongs to, grouped by organization, so jobs and
open windows survive a switch. Desktop's organization picker is a view preference.

**Personal homes stay personal.** There is one home per account on the platform homes share, managed by Root.
Organizations own Shared Folders.

**Storage allocations carve capacity out of physical pools.** `storage_allocations(organization, volume)` gives
an organization a budget on one pool:

- A top-level allocation may be unlimited (bounded by the pool). A child allocation must have a limit, and its
  parent organization needs an allocation on the same pool.
- A child's limit is subtracted from the parent's direct budget (`limit − delegated`). This is checked under a
  row lock on the parent allocation, so concurrent grants can never over-allocate.
- A limit can never go below `delegated + used + reserved`.
- Each allocation has a ledger row (`storage_quota` scope `allocation`) whose limit is its direct budget. A trigger
  rolls every share's used/reserved change into its allocation. Raising a reservation over the budget fails the
  reservation (`QuotaExceeded{allocation}`). Writes therefore check home or share, quota folders and the
  allocation in one transaction.
- Shares reference their allocation with a composite FK `(allocation_id, organization_id, volume)`, so a share
  can never sit on another organization's or pool's budget. Organizations with shares or allocations cannot be
  deleted (`ON DELETE RESTRICT`).

**Audit and notices carry the organization.** `identity_admin_events`, `storage_events` and `files_events` have
`organization_id`. Storage fills it by trigger from the target share, folder or allocation, and Files fills it
from the share key. Log Center shows Root every event, and shows an organization admin only the events of
organizations they administer. Quota notices for shares, folders and allocations go to the direct admins of the
owning organization; the homes share still notifies Root.

**Adoption (expand only).** Identity SQL 012 creates one default organization (`is_default`, children allowed).
Every existing account joins it (System Admins as `admin`) and it receives every user group. Storage SQL 006 gives
the default organization an unlimited allocation on each ZFS/Btrfs pool and moves every ordinary share into it.
A NAS keeps working unchanged, as one organization.

## Consequences

- Usernames stay global (sign-in), so adding or creating a user can reveal that a username exists. This is
  accepted and documented.
- Organization admins create standard accounts inside their organization and add existing accounts by username.
  Disabling, deleting and resetting the password of an account stay platform operations (Root/user-admin).
- Re-parenting, cross-organization sharing and moving a share between organizations are not in v1.
- Every app reviewed against the app standard adds row 27 (tenancy).
- No "active organization" in the session and no organization switcher in Desktop: an app that needs one passes
  the organization as a path or query selector, and the server authorizes it on that request. Files lists the
  Shared Folders of every organization the user belongs to, grouped by organization.
- Files jobs carry no organization column: a job re-checks share access (membership included) at run time, so a
  member who leaves cannot finish a copy into the organization's shares.
- UI (2026-10-31): Account Center → *Tổ chức* (tree, members, organization users, organization groups);
  Storage Manager → *Tổ chức* (allocations per pool) and an owning organization when creating a share; Log Center
  filters by organization; Notification Center renders `organization.joined/left/role` and allocation quota notices.
