import { runtimeConfig } from '../../config/runtime';
import { CSRF_HEADER, CSRF_HEADER_VALUE } from '../../features/auth/authConstants';
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
  body?: BodyInit,
  jsonContentType = true
) =>
  fetch(path, {
    method,
    // Same-origin: the session cookie rides along automatically, and the BFF
    // injects the bearer token upstream itself — this client never holds or
    // sends one.
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      'X-Org-Id': orgRef,
      // For FormData we omit Content-Type so the browser sets the multipart
      // boundary; otherwise JSON bodies get an explicit content type.
      ...(body && jsonContentType
        ? { 'Content-Type': 'application/json' }
        : {}),
      // The BFF requires this on every mutating request (GET/HEAD/OPTIONS are
      // exempt) as its CSRF defense — safe because there is no CORS layer, so
      // only same-origin script can set it.
      ...(method !== 'GET' ? { [CSRF_HEADER]: CSRF_HEADER_VALUE } : {}),
    },
    ...(body ? { body } : {}),
  });

/**
 * Issues a platform-api REST request through the BFF's same-origin proxy
 * (which forwards it to BML), plus X-Org-Id so BML can resolve/validate the
 * active organization. A 401 here means the BFF's session itself is gone
 * (expired/not authenticated) — the BFF already renews a near-expiry OIDC
 * session server-side before ever proxying, so there is nothing for this
 * client to refresh or retry; it surfaces as ApiError('UNAUTHORIZED') like
 * any other failed request.
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
    response = await sendOnce(method, path, orgRef, body, jsonContentType);
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
