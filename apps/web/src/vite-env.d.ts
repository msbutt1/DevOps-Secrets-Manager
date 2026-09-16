/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL of the API; defaults to /api, which the dev server and nginx proxy. */
  readonly VITE_API_BASE_URL?: string;
  /** Shown as a banner on every page, for demo deployments (see docs/operations.md). */
  readonly VITE_DEMO_BANNER?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
