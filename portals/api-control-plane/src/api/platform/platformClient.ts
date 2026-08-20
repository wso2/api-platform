import { runtimeConfig } from '../../config/runtime';
import { getPlatformToken, refreshPlatformToken } from '../client';
import { ApiError, type ApiErrorCode } from '../types/errors';

/** True when read flows should go to platform-api REST (via BML). */
export const usePlatformApi = () => Boolean(runtimeConfig.platformApiBaseUrl);

/**
 * Full platform-api base every client route is built on (host + version prefix,
 * e.g. `.../platform-api-endpoint/api/v1`). Composed from the
 * `PLATFORM_API_BASE_URL` host base and the `PLATFORM_API_VERSION` segment
 * (defaults to "v1"), so bumping platform-api's REST version is a config change
 * rather than a code change.
 */
export const PLATFORM_API_BASE = `${runtimeConfig.platformApiBaseUrl}/api/${runtimeConfig.platformApiVersion}`;

const codeForStatus = (status: number): ApiErrorCode => {
  if (status === 401) return 'UNAUTHORIZED';
  if (status === 403) return 'FORBIDDEN';
  if (status === 404) return 'NOT_FOUND';
  if (status === 409) return 'CONFLICT';
  if (status >= 500) return 'SERVER_ERROR';
  return 'UNKNOWN';
};

const sendOnce = (
  method: string,
  path: string,
  orgRef: string,
  token: string | undefined,
  body?: BodyInit,
  jsonContentType = true
) =>
  fetch(path, {
    method,
    headers: {
      Accept: 'application/json',
      'X-Org-Id': orgRef,
      // For FormData we omit Content-Type so the browser sets the multipart
      // boundary; otherwise JSON bodies get an explicit content type.
      ...(body && jsonContentType
        ? { 'Content-Type': 'application/json' }
        : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    ...(body ? { body } : {}),
  });

/**
 * Issues a platform-api REST request through BML. Sends the RAW IdP access
 * token (BML verifies it, then either forwards it or mints an org-scoped token)
 * plus X-Org-Id so BML can resolve/validate the active organization.
 *
 * The IdP access token expires and the provider does not refresh it eagerly, so
 * on a 401 we refresh the token once (via the registered refresher) and retry —
 * otherwise calls start failing a few minutes into a session. The token
 * provider/refresher are registered by the active auth adapter, so this client
 * stays decoupled from the specific IdP SDK (Asgardeo / Thunder / local-file).
 */
async function platformRequest<T>(
  method: string,
  path: string,
  orgRef: string,
  body?: BodyInit,
  jsonContentType = true
): Promise<T> {
  let response: Response;
  try {
    response = await sendOnce(
      method,
      path,
      orgRef,
      await getPlatformToken(),
      body,
      jsonContentType
    );
    if (response.status === 401) {
      try {
        const refreshed = await refreshPlatformToken();
        if (refreshed !== undefined) {
          response = await sendOnce(
            method,
            path,
            orgRef,
            refreshed,
            body,
            jsonContentType
          );
        }
      } catch {
        // refresh failed — fall through and surface the original 401 below
      }
    }
  } catch (error) {
    throw new ApiError(
      error instanceof Error ? error.message : 'Network error',
      'NETWORK_ERROR'
    );
  }

  if (!response.ok) {
    let message = `platform-api request failed (${response.status})`;
    try {
      const errBody = (await response.json()) as {
        message?: string;
        description?: string;
        error?: string;
      };
      message =
        errBody.description || errBody.message || errBody.error || message;
    } catch {
      // non-JSON error body
    }
    throw new ApiError(
      message,
      codeForStatus(response.status),
      response.status
    );
  }

  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export const platformGet = <T>(path: string, orgRef: string): Promise<T> =>
  platformRequest<T>('GET', path, orgRef);

export const platformPost = <T>(
  path: string,
  orgRef: string,
  body: unknown
): Promise<T> => platformRequest<T>('POST', path, orgRef, JSON.stringify(body));

/** POSTs multipart/form-data (e.g. OpenAPI import). Lets the browser set boundary. */
export const platformPostForm = <T>(
  path: string,
  orgRef: string,
  form: FormData
): Promise<T> => platformRequest<T>('POST', path, orgRef, form, false);

export const platformPut = <T>(
  path: string,
  orgRef: string,
  body: unknown
): Promise<T> => platformRequest<T>('PUT', path, orgRef, JSON.stringify(body));

export const platformDelete = <T>(path: string, orgRef: string): Promise<T> =>
  platformRequest<T>('DELETE', path, orgRef);
