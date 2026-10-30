# ADR 0003 — App identity, naming and repository policy

Accepted 2026-10-30. Branch platform-app-naming-v1.

The active repository is minhtribtc/MinhAnOS, a monorepo. desktop-v2-ipados is an existing branch, not the
repository name used by this workspace. Keep the root repository name: it includes platform, apps, SDK and contracts.
Technical module names minhanos-* identify app workspaces and possible independently published products;
they do not imply eight new repositories have been created. Revisit multi-repo when ownership/release cycles justify it.

| Component | Canonical product name | Technical module | Stable App ID |
|---|---|---|---|
| Desktop | MinhAnOS Desktop | minhanos-desktop | com.minhanos.desktop |
| Files | MinhAnOS Files | minhanos-files | com.minhanos.files |
| Accounts | Account Center | minhanos-accounts | com.minhanos.account-center |
| Settings | Control Center | minhanos-settings | com.minhanos.settings |
| Storage | Storage Manager | minhanos-storage | com.minhanos.storage |
| App registry/management | App Center | minhanos-app-center | com.minhanos.app-center |
| Containers | Container Manager | minhanos-containers | com.minhanos.containers |
| Monitoring | Resource Monitor | minhanos-monitor | com.minhanos.monitor |

Choose MinhAnOS Files as the canonical brand to keep product naming consistent. File Station and MinhAnOS Drive
remain current UI names/aliases during the transition. This is a naming decision, not a legal/trademark clearance.
No broad UI rename is bundled with this contract/workspace change. The standalone frontend workspace name changes
from minhanos-drive to minhanos-files. Shared package @minhanos/files-app remains unchanged.

Machine-readable authority: contracts/apps/catalog.v1.json, validated against catalog.v1.schema.json.
Each app records implementation status, legacy registry aliases and actual source paths. Account Center and
Control Center are partial, still hosted in Desktop Settings; App Center is partial registry/discovery, not an installer.
Storage Manager, Container Manager and Resource Monitor are planned. Do not rename Overview to Resource Monitor:
current process/API metrics do not establish whole-host monitoring. An ID reservation does not enable a feature.
The catalog is for first-party product identity, not third-party app discovery or backend grants.

Identity/compatibility:

- App ID is immutable identity; product name may change without changing identity or user data.
- Legacy registry IDs files/apps/settings remain in URLs/layouts. Alias resolution must map to canonical ID
  before future grants/authorization; duplicate aliases are rejected. The new catalog is not yet a runtime resolver.
- Keep apps/drive, internal/drive, compose service drive, image tags, volumes, /api/drive, ports 8081/8082,
  /opt/minhanos-next and Git remote unchanged. Rename these only through a separately tested migration.
- Technical package IDs are not mechanically derived from display names or App IDs; account-center maps to
  minhanos-accounts explicitly. Shared packages retain the @minhanos scope.
- Canonical display names in this catalog are product metadata; per-app localized UI still belongs in app catalogs.
  UI names are not authorization identifiers, and capabilities in manifests remain requested permissions.

Presentation terminology:

- Desktop is the product; Desktop Shell hosts launcher, dock, window manager and notifications.
- Workspace is a user's working context, not another app/repository by default.
- classic/touch are future Shell variants. light/dark/system are appearance preferences owned by the platform.
  focus/windows remain workspace layout modes. These three axes must not be collapsed into one theme field.
- Prefer shell/variants/classic and shell/variants/touch when implemented. Keep palette/theme tokens in packages/ui.
- Control Center names the settings application; the small quick-toggle panel should be called Quick Settings
  when its strings are migrated to desktop i18n. Existing UI labels remain in this phase.

CI checks JSON Schema, unique IDs/module names/aliases, traversal-free source paths, existing paths, manifest
membership and actual Desktop/Files workspace names. It does not pretend reserved apps have runtime manifests,
entrypoints, services or grants. Continue identity adoption and app delegation after this naming alignment.


Historical compatibility checks reject removal of existing App IDs and removal/reassignment of legacy aliases.
Display name changes remain allowed. First catalog introduction has no historical baseline; negative fixtures cover the guard.
