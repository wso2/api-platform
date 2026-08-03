/*
 * Local dev override for `choreo.common.config.js`.
 *
 * The shared legacy common config injects ~30 hosted *.choreo.dev service URLs
 * (PROJECT_API_BASE_URL, ORGANIZATION_API_URL, USERS_MANAGEMENT_API_URL, ...).
 * For a local cluster we want NONE of those — all traffic goes through
 * platform-api — so this override is intentionally empty.
 */
