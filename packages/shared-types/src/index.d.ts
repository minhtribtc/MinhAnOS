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
    | "uploads.cancel";
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
