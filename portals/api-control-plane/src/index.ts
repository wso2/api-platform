// Public entry point for consuming this app as a workspace-linked package (see
// docs/remote-app-wrapper-app-architecture.md). Because package.json's "main"
// points here (at TS source, not a build output), a host importing
// "management-portal" resolves straight through to these actual files —
// hot-reloading edits across both apps in a single dev server, with no
// separate "build management-portal, then run the host" step.
export { default as App } from './App';
export { loadRuntimeConfigScripts } from './config/loadRuntimeConfigScripts';
