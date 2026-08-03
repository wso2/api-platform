export const AUTH_SESSION_STORAGE_KEY = 'oxygen-console-session';
export const AUTH_RETURN_TO_STORAGE_KEY = 'oxygen-console-return-to';
// Persistent (localStorage) marker set once a session has been established in
// this browser. Used to decide whether a silent (prompt=none) sign-in is worth
// attempting on load — there is nothing to restore for a first-time visitor.
export const AUTH_SESSION_HINT_KEY = 'oxygen-console-had-session';
export const DEFAULT_SESSION_DURATION_MS = 8 * 60 * 60 * 1000;
export const SESSION_LOAD_TIMEOUT_MS = 15000;
// Silent sign-in must be bounded so a stalled prompt=none iframe (e.g. no IdP
// session, or a redirect/cookie mismatch) can never hang the protected route.
export const SILENT_SIGN_IN_TIMEOUT_MS = 8000;
