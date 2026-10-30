/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL of the v1 REST API (default: same-origin /api/v1). */
  readonly VITE_API_V1_URL?: string;
  /** WebSocket endpoint: absolute URL, path, or host (default: /api/v1/ws). */
  readonly VITE_WS_URL?: string;
  /** Overrides the version shown in Settings. */
  readonly VITE_APP_VERSION?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
