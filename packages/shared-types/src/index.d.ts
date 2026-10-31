// Generated from contracts/*.schema.json. Run npm run contracts:generate. Do not edit.
export interface AppManifest {
  manifestVersion: "1";
  id: string;
  version: string;
  sdk: string;
  entrypoint: string;
  /**
   * @minItems 1
   */
  display: ["window" | "maximized" | "fullscreen", ...("window" | "maximized" | "fullscreen")[]];
  fileHandlers?: {
    /**
     * @minItems 1
     */
    mimeTypes: [string, ...string[]];
    /**
     * @minItems 1
     */
    actions: ["open" | "preview" | "edit", ...("open" | "preview" | "edit")[]];
  }[];
  capabilities: string[];
  lifecycle?: {
    version: "1";
    supports: ("foreground" | "background" | "requestClose")[];
  };
  navigation?: {
    id: string;
    labelKey: string;
    path: string;
    requiredPermission?: string;
  }[];
  nameKey?: string;
  settingsIntegration?: {
    id: string;
    labelKey: string;
    path: string;
    requiredPermission?: string;
  }[];
}

export interface ApiErrorEnvelope {
  error: {
    code: string;
    message: string;
    requestId: string;
    details: {
      [k: string]: unknown;
    };
  };
}

export interface LegacyErrorEnvelope {
  code: string;
  error: string;
}

export interface AccountProfile {
  locale: "vi-VN" | "en-US";
  theme: "light" | "dark" | "system";
  timeZone: string;
}

export interface FileResource {
  fileId: string;
  name: string;
  parentId: string | null;
  kind: "file" | "folder";
  mimeType: string | null;
  size: number;
  version: number;
  etag: string;
  modifiedAt: string;
  permissions: ("read" | "write" | "delete" | "share")[];
}

export interface FileAcl {
  fileId: string;
  version: number;
  etag: string;
  ownerId: string;
  inherit: boolean;
  /**
   * @maxItems 1000
   */
  entries: {
    principal: string;
    allow: ("read" | "write" | "delete" | "share")[];
    deny: ("read" | "write" | "delete" | "share")[];
  }[];
}

export interface Job {
  id: string;
  type: string;
  state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
  progress: {
    completed: number;
    total: number | null;
    unit: "bytes" | "items" | "steps";
  };
  attempt: number;
  maxAttempts: number;
  createdAt: string;
  updatedAt: string;
  error: {
    code: string;
    message: string;
    requestId: string;
    details: {
      [k: string]: unknown;
    };
  } | null;
}

export interface UploadSession {
  id: string;
  state: "created" | "uploading" | "completing" | "completed" | "cancelled" | "expired" | "failed";
  size: number;
  partSize: number;
  receivedParts: number[];
  checksum: string;
  expiresAt: string;
  fileId: string | null;
}

export interface AuthorizationCheck {
  action:
    | "files.read"
    | "files.write"
    | "files.system.read"
    | "files.system.write"
    | "jobs.read"
    | "jobs.create"
    | "jobs.cancel"
    | "uploads.create"
    | "uploads.write"
    | "uploads.cancel"
    | "identity.self.read"
    | "identity.self.write"
    | "identity.users.read"
    | "identity.users.write"
    | "identity.groups.read"
    | "identity.groups.write"
    | "identity.roles.read"
    | "identity.roles.write"
    | "identity.apps.read"
    | "identity.apps.write"
    | "identity.sessions.read"
    | "identity.sessions.revoke"
    | "identity.password.change"
    | "identity.audit.read";
  resource: string;
  context: {
    appId: string;
  };
}

export type PlatformEvent = {
  [k: string]: unknown;
} & {
  eventId: string;
  name: "files.created.v1" | "files.updated.v1" | "files.deleted.v1" | "identity.user.disabled.v1";
  schemaVersion: 1;
  occurredAt: string;
  source: string;
  subject: string;
  correlationId: string;
  data: {
    [k: string]: unknown;
  };
};

export interface IdentityUser {
  id: string;
  username: string;
  displayName: string;
  status: "active" | "disabled" | "deleted";
  version: number;
  passwordChangeRequired?: boolean;
}

export interface IdentityGroup {
  id: string;
  key: string;
  name: string;
  kind: "system" | "user";
  version: number;
  organizationId?: string | null;
  memberCount?: number;
}

export interface IdentityRole {
  id: string;
  key: string;
  name: string;
  protected: boolean;
  permissions: string[];
  version: number;
}

export interface IdentityPermission {
  key: string;
  descriptionKey: string;
  resourceTypes: string[];
}

export interface IdentityRoleBinding {
  id: string;
  roleId: string;
  subject: {
    kind: "user" | "group";
    id: string;
  };
  scope: {
    kind: "system" | "user" | "storage";
    id: string | null;
  };
  version: number;
}

export interface IdentitySession {
  id: string;
  userId: string;
  realm: "desktop" | "files";
  current: boolean;
  createdAt: string | null;
  lastSeenAt: string | null;
  expiresAt: string;
  deviceHint: string | null;
}

export interface IdentityCredentialRef {
  id: string;
  userId: string;
  kind: "password" | "webauthn" | "totp" | "recovery";
  lastChangedAt: string | null;
  state: "active" | "revoked" | "unsupported";
}

export interface IdentityLink {
  id: string;
  userId: string;
  issuer: string;
  subject: string;
  createdAt: string;
}

export interface IdentityLinuxIdentity {
  userId: string;
  nodeId: string;
  uid: number | null;
  primaryGid: number | null;
  supplementaryGids: number[];
  state: "unmapped" | "reserved" | "ready" | "failed" | "disabled" | "retired";
  version: number;
}

export interface IdentityAuditEvent {
  id: string;
  actorId: string | null;
  action: string;
  targetId: string | null;
  outcome: "success" | "failure" | "denied";
  occurredAt: string;
  requestId: string;
  details: {
    reasonCode?: string;
    changedFields?: string[];
    realm?: "desktop" | "files";
    count?: number;
  };
  correlationId?: string | null;
}

export interface IdentityCreateUser {
  username: string;
  displayName: string;
  password: string;
}

export interface IdentityPatchUser {
  displayName?: string;
}

export interface IdentityChangePassword {
  currentPassword: string;
  newPassword: string;
}

export interface IdentityUserPage {
  items: {
    id: string;
    username: string;
    displayName: string;
    status: "active" | "disabled" | "deleted";
    version: number;
    passwordChangeRequired?: boolean;
  }[];
  nextCursor: string | null;
}

export interface IdentitySessionPage {
  items: {
    id: string;
    userId: string;
    realm: "desktop" | "files";
    current: boolean;
    createdAt: string | null;
    lastSeenAt: string | null;
    expiresAt: string;
    deviceHint: string | null;
  }[];
  nextCursor: string | null;
}

export interface IdentityMutationResult {
  accepted: boolean;
}

export interface IdentityGroupPage {
  items: {
    id: string;
    key: string;
    name: string;
    kind: "system" | "user";
    version: number;
    organizationId?: string | null;
    memberCount?: number;
  }[];
  nextCursor: string | null;
}

export interface IdentityRolePage {
  items: {
    id: string;
    key: string;
    name: string;
    protected: boolean;
    permissions: string[];
    version: number;
  }[];
  nextCursor: string | null;
}

export interface IdentityPermissionPage {
  items: {
    key: string;
    descriptionKey: string;
    resourceTypes: string[];
  }[];
  nextCursor: string | null;
}

export interface IdentityAuditPage {
  items: {
    id: string;
    actorId: string | null;
    action: string;
    targetId: string | null;
    outcome: "success" | "failure" | "denied";
    occurredAt: string;
    requestId: string;
    details: {
      reasonCode?: string;
      changedFields?: string[];
      realm?: "desktop" | "files";
      count?: number;
    };
    correlationId?: string | null;
  }[];
  nextCursor: string | null;
}

export interface IdentityCredentialPage {
  items: {
    id: string;
    userId: string;
    kind: "password" | "webauthn" | "totp" | "recovery";
    lastChangedAt: string | null;
    state: "active" | "revoked" | "unsupported";
  }[];
  nextCursor: string | null;
}

export interface IdentityCreateGroup {
  key: string;
  name: string;
}

export interface IdentityPatchGroup {
  name?: string;
}

export interface IdentityGroupMember {
  userId: string;
}

export interface IdentityCreateRole {
  key: string;
  name: string;
  permissions: string[];
}

export interface IdentityCreateRoleBinding {
  roleId: string;
  subject: {
    kind: "user" | "group";
    id: string;
  };
  scope: {
    kind: "system" | "user" | "storage";
    id: string | null;
  };
}

export interface IdentityApplication {
  appId: string;
  displayName: string;
  allowed: boolean;
}

export interface IdentityApplicationPage {
  items: {
    appId: string;
    displayName: string;
    allowed: boolean;
  }[];
  nextCursor: string | null;
}

export interface IdentityReauthenticate {
  password: string;
}

export interface IdentitySetRole {
  role: "system-admin" | "user-admin" | "standard-user";
}

export interface IdentityGroupMemberPage {
  items: {
    userId: string;
  }[];
  nextCursor: string | null;
}

export interface IdentityRoleBindingPage {
  items: {
    id: string;
    roleId: string;
    subject: {
      kind: "user" | "group";
      id: string;
    };
    scope: {
      kind: "system" | "user" | "storage";
      id: string | null;
    };
    version: number;
  }[];
  nextCursor: string | null;
}

export interface IdentityAccess {
  userId: string;
  baseRole: "system-admin" | "user-admin" | "standard-user";
  permissions: string[];
  hostOperator: boolean;
}

export interface IdentityResetPassword {
  newPassword: string;
  requireChange?: boolean;
}

export interface IdentityAppAccessRule {
  id: string;
  appId: string;
  effect: "deny";
  subject: {
    kind: "user" | "group" | "role";
    id: string;
  };
  version: number;
}

export interface IdentityAppAccessPage {
  items: {
    id: string;
    appId: string;
    effect: "deny";
    subject: {
      kind: "user" | "group" | "role";
      id: string;
    };
    version: number;
  }[];
  nextCursor: string | null;
}

export interface IdentityCreateAppAccess {
  subject: {
    kind: "user" | "group" | "role";
    id: string;
  };
}

export interface IdentityOrganization {
  id: string;
  name: string;
  parentId: string | null;
  depth: number;
  status: "active" | "suspended";
  allowChildren: boolean;
  isDefault: boolean;
  role: "root" | "admin" | "manager" | "member";
  memberCount: number;
  version: number;
}

export interface IdentityOrganizationPage {
  /**
   * @maxItems 100
   */
  items: {
    id: string;
    name: string;
    parentId: string | null;
    depth: number;
    status: "active" | "suspended";
    allowChildren: boolean;
    isDefault: boolean;
    role: "root" | "admin" | "manager" | "member";
    memberCount: number;
    version: number;
  }[];
  nextCursor: string | null;
}

export interface IdentityCreateOrganization {
  name: string;
  parentId?: string | null;
  allowChildren?: boolean;
}

export interface IdentityPatchOrganization {
  name?: string;
  status?: "active" | "suspended";
  allowChildren?: boolean;
}

export interface IdentityOrganizationMember {
  userId: string;
  username: string;
  displayName: string;
  status: "active" | "disabled" | "deleted";
  role: "admin" | "member";
}

export interface IdentityOrganizationMemberPage {
  /**
   * @maxItems 100
   */
  items: {
    userId: string;
    username: string;
    displayName: string;
    status: "active" | "disabled" | "deleted";
    role: "admin" | "member";
  }[];
  nextCursor: string | null;
}

export interface IdentityAddOrganizationMember {
  username: string;
  role: "admin" | "member";
}

export interface IdentitySetOrganizationMemberRole {
  role: "admin" | "member";
}

export interface IdentityCreateOrganizationUser {
  username: string;
  displayName: string;
  password: string;
  role: "admin" | "member";
}

export interface IdentityOrganizationMemberResult {
  accepted: true;
  userId: string;
}

export interface StorageVolume {
  name: string;
  kind: "zfs" | "btrfs" | "directory";
  managed: boolean;
  hardQuota: boolean;
  snapshots: boolean;
  online: boolean;
}

export interface StorageShare {
  id: string;
  key: string;
  name: string;
  volume: string;
  kind: "share" | "homes";
  version: number;
  usage: {
    limitBytes: number | null;
    usedBytes: number;
    reservedBytes: number;
    measured: boolean;
  };
  physicalBytes: number | null;
  organizationId: string | null;
  organizationName: string | null;
}

export interface StorageHome {
  userId: string;
  username: string;
  displayName: string;
  share: string;
  version: number;
  usage: {
    limitBytes: number | null;
    usedBytes: number;
    reservedBytes: number;
    measured: boolean;
  };
}

export interface StorageCreateShare {
  key: string;
  name: string;
  volume: string;
  limitBytes: number | null;
  hostsHomes?: boolean;
  organizationId?: string;
}

export interface StorageSetQuota {
  limitBytes: number | null;
}

export interface StorageSettings {
  defaultHomeLimitBytes: number | null;
  version: number;
}

export interface StorageSettingsInput {
  defaultHomeLimitBytes: number | null;
}

export interface StorageAccepted {
  accepted: true;
}

export interface StorageVolumePage {
  items: {
    name: string;
    kind: "zfs" | "btrfs" | "directory";
    managed: boolean;
    hardQuota: boolean;
    snapshots: boolean;
    online: boolean;
  }[];
  agentError: boolean;
}

export interface StorageSharePage {
  items: {
    id: string;
    key: string;
    name: string;
    volume: string;
    kind: "share" | "homes";
    version: number;
    usage: {
      limitBytes: number | null;
      usedBytes: number;
      reservedBytes: number;
      measured: boolean;
    };
    physicalBytes: number | null;
    organizationId: string | null;
    organizationName: string | null;
  }[];
}

export interface StorageHomePage {
  items: {
    userId: string;
    username: string;
    displayName: string;
    share: string;
    version: number;
    usage: {
      limitBytes: number | null;
      usedBytes: number;
      reservedBytes: number;
      measured: boolean;
    };
  }[];
}

export interface StorageShareAccess {
  version: number;
  /**
   * @maxItems 200
   */
  entries: {
    subjectType: "user" | "group";
    subjectId: string;
    name?: string;
    access: "read" | "write" | "deny";
  }[];
}

export interface StorageShareAccessInput {
  /**
   * @maxItems 200
   */
  entries: {
    subjectType: "user" | "group";
    subjectId: string;
    access: "read" | "write" | "deny";
  }[];
}

export interface StorageSubjectPage {
  items: {
    type: "user" | "group";
    id: string;
    name: string;
    detail: string;
  }[];
}

export interface StorageQuotaFolder {
  id: string;
  shareId: string;
  path: string;
  version: number;
  usage: {
    limitBytes: number | null;
    usedBytes: number;
    reservedBytes: number;
    measured: boolean;
  };
}

export interface StorageQuotaFolderPage {
  /**
   * @maxItems 100
   */
  items: {
    id: string;
    shareId: string;
    path: string;
    version: number;
    usage: {
      limitBytes: number | null;
      usedBytes: number;
      reservedBytes: number;
      measured: boolean;
    };
  }[];
}

export interface StorageCreateQuotaFolder {
  path: string;
  limitBytes: number | null;
}

export interface StorageAllocation {
  id: string;
  organizationId: string;
  volume: string;
  parentId: string | null;
  limitBytes: number | null;
  delegatedBytes: number;
  usedBytes: number;
  reservedBytes: number;
  version: number;
}

export interface StorageCreateAllocation {
  volume: string;
  limitBytes: number | null;
}

export interface StorageOrganizationPage {
  /**
   * @maxItems 500
   */
  items: {
    id: string;
    name: string;
    parentId: string | null;
    status: "active" | "suspended";
    role: "root" | "admin" | "manager";
    /**
     * @maxItems 100
     */
    allocations: {
      id: string;
      organizationId: string;
      volume: string;
      parentId: string | null;
      limitBytes: number | null;
      delegatedBytes: number;
      usedBytes: number;
      reservedBytes: number;
      version: number;
    }[];
  }[];
}

export interface NotificationItem {
  id: string;
  source: string;
  kind: string;
  severity: "info" | "warning" | "critical";
  mandatory: boolean;
  params: {
    [k: string]: string | number;
  };
  createdAt: string;
  readAt: string | null;
}

export interface NotificationMarkRead {
  /**
   * @minItems 1
   * @maxItems 100
   */
  ids?: [string, ...string[]];
  upTo?: string;
}

export interface NotificationMarkReadResult {
  updated: number;
  unreadCount: number;
}

export interface NotificationPage {
  items: {
    id: string;
    source: string;
    kind: string;
    severity: "info" | "warning" | "critical";
    mandatory: boolean;
    params: {
      [k: string]: string | number;
    };
    createdAt: string;
    readAt: string | null;
  }[];
  nextCursor: string | null;
  unreadCount: number;
}

export interface LogSource {
  id: "identity" | "storage" | "files";
  appId: string;
  retentionDays: number | null;
}

export interface LogEvent {
  id: string;
  source: "identity" | "storage" | "files";
  occurredAt: string;
  action: string;
  outcome: "success" | "denied" | "failure";
  actor: {
    id: string;
    username: string;
  } | null;
  target: string | null;
  targetName: string | null;
  requestId: string;
  correlationId: string | null;
  details: {};
  organizationId: string | null;
}

export interface LogSourceList {
  items: {
    id: "identity" | "storage" | "files";
    appId: string;
    retentionDays: number | null;
  }[];
}

export interface LogEventPage {
  items: {
    id: string;
    source: "identity" | "storage" | "files";
    occurredAt: string;
    action: string;
    outcome: "success" | "denied" | "failure";
    actor: {
      id: string;
      username: string;
    } | null;
    target: string | null;
    targetName: string | null;
    requestId: string;
    correlationId: string | null;
    details: {};
    organizationId: string | null;
  }[];
  nextCursor: string | null;
}

export interface FilesEntry {
  name: string;
  path: string;
  kind: "file" | "folder" | "symlink" | "special";
  size: number;
  modified: string;
  mode: string;
  uid: number;
  gid: number;
  etag: string;
}

export interface FilesJob {
  id: string;
  type: "files.copy";
  state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
  progress: {
    completed: number;
    total: number | null;
    unit: "bytes";
  };
  attempt: number;
  maxAttempts: number;
  createdAt: string;
  updatedAt: string;
  error: {
    code: string;
    message: string;
    requestId: string;
    details: {
      [k: string]: unknown;
    };
  } | null;
}

export interface FilesJobList {
  /**
   * @maxItems 20
   */
  items:
    | []
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ]
    | [
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        },
        {
          id: string;
          type: "files.copy";
          state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
          progress: {
            completed: number;
            total: number | null;
            unit: "bytes";
          };
          attempt: number;
          maxAttempts: number;
          createdAt: string;
          updatedAt: string;
          error: {
            code: string;
            message: string;
            requestId: string;
            details: {
              [k: string]: unknown;
            };
          } | null;
        }
      ];
}

export interface FilesListPage {
  path: string;
  /**
   * @maxItems 1000
   */
  entries: {
    name: string;
    path: string;
    kind: "file" | "folder" | "symlink" | "special";
    size: number;
    modified: string;
    mode: string;
    uid: number;
    gid: number;
    etag: string;
  }[];
  nextCursor: string | null;
}

export interface FilesText {
  content: string;
  etag: string;
}

export interface FilesTrashItem {
  id: string;
  path: string;
  name: string;
  deleted: string;
}

/**
 * @maxItems 10000
 */
export type FilesTrashList = {
  id: string;
  path: string;
  name: string;
  deleted: string;
}[];

export interface FilesNameInput {
  name: string;
}

export interface FilesDestinationInput {
  destination: string;
}

export interface FilesRestoreInput {
  id: string;
}

export interface FilesRemoveInput {
  confirm: true;
}

export interface FilesSaveTextInput {
  content: string;
}

export interface FilesWriteResult {
  ok: true;
  item?:
    | {
        name: string;
        path: string;
        kind: "file" | "folder" | "symlink" | "special";
        size: number;
        modified: string;
        mode: string;
        uid: number;
        gid: number;
        etag: string;
      }
    | {
        id: string;
        path: string;
        name: string;
        deleted: string;
      };
  backup?: string;
  job?: {
    id: string;
    type: "files.copy";
    state: "queued" | "running" | "succeeded" | "failed" | "cancelled";
    progress: {
      completed: number;
      total: number | null;
      unit: "bytes";
    };
    attempt: number;
    maxAttempts: number;
    createdAt: string;
    updatedAt: string;
    error: {
      code: string;
      message: string;
      requestId: string;
      details: {
        [k: string]: unknown;
      };
    } | null;
  };
}

export interface FilesRootList {
  /**
   * @maxItems 1000
   */
  roots: {
    id: string;
    kind: "home" | "share" | "system";
    name?: string;
    access: "read" | "write";
    organizationId?: string;
    organization?: string;
  }[];
  homes: boolean;
  maxUploadBytes: number;
  maxTextBytes: number;
  quota?: {
    limitBytes: number | null;
    usedBytes: number;
    reservedBytes: number;
    measured: boolean;
  };
}

export interface FilesQuotaFolderList {
  /**
   * @maxItems 1000
   */
  items: {
    id: string;
    shareId: string;
    path: string;
    version: number;
    usage: {
      limitBytes: number | null;
      usedBytes: number;
      reservedBytes: number;
      measured: boolean;
    };
  }[];
}

export interface FilesSession {
  user: {
    id: string;
    username: string;
    displayName: string;
    role: "admin" | "user";
  };
  csrfToken: string;
  passwordChangeRequired: boolean;
}

export interface FilesLoginInput {
  username: string;
  password: string;
}

export interface FilesOk {
  ok: true;
}

export interface FilesHealth {
  app: "MinhAnOS Files";
  version: string;
  independent: boolean;
}

export interface FilesEmptyInput {}

export interface DesktopSession {
  user: {
    id: string;
    username: string;
    displayName: string;
    role: "admin" | "user";
  };
  csrfToken: string;
  passwordChangeRequired: boolean;
}

export interface DesktopLoginInput {
  username: string;
  password: string;
}

/**
 * @maxItems 64
 */
export type DesktopAppList = {
  id: string;
  icon: string;
  color: string;
}[];

export interface DesktopPreferences {
  theme: "light" | "dark";
  wallpaper: "mountain" | "midnight" | "sage";
  mode: "focus" | "windows";
  /**
   * @maxItems 32
   */
  windows: {
    id: string;
    x: number;
    y: number;
    width: number;
    height: number;
    minimized: boolean;
    maximized: boolean;
  }[];
  notes: string;
  /**
   * @maxItems 32
   */
  pinned: string[];
}

export interface DesktopExport {
  app: "com.minhanos.desktop";
  exportVersion: "1";
  exportedAt: string;
  preferences: {
    theme: "light" | "dark";
    wallpaper: "mountain" | "midnight" | "sage";
    mode: "focus" | "windows";
    /**
     * @maxItems 32
     */
    windows: {
      id: string;
      x: number;
      y: number;
      width: number;
      height: number;
      minimized: boolean;
      maximized: boolean;
    }[];
    notes: string;
    /**
     * @maxItems 32
     */
    pinned: string[];
  };
}

export interface DesktopSystemStatus {
  version: string;
  hostname: string;
  os: string;
  architecture: string;
  goVersion: string;
  cpus: number;
  goroutines: number;
  heapBytes: number;
  uptimeSeconds: number;
  diskTotalBytes: number;
  diskFreeBytes: number;
  database: "connected" | "disconnected";
}

export interface DesktopOk {
  ok: true;
}

export interface DesktopHealth {
  status: "ok";
  version: string;
}

export interface DesktopSetupStatus {
  required: boolean;
}

export interface DesktopSetupInput {
  setupCode: string;
  username: string;
  displayName: string;
  password: string;
  organizationName: string;
}

export interface DesktopSetupResult {
  user: {
    username: string;
    displayName: string;
  };
  organization: string;
}
