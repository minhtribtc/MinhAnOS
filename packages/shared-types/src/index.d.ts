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
