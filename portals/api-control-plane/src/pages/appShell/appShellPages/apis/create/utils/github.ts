/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { isValidUrl } from '../../utils/developEdit';

/**
 * Public GitHub, read straight from the browser.
 *
 * Anonymous reads only: `api.github.com` allows cross-origin requests and
 * needs no key for a public repository, but it also allows only 60 of them per
 * hour per address; which is why the whole tree is taken in one request
 * rather than a call per expanded folder, and why `rateLimited` is a verdict
 * the UI has to be able to show.
 */

const API_BASE_URL = 'https://api.github.com';
const RAW_BASE_URL = 'https://raw.githubusercontent.com';

/** What a repository URL names. Branch and path come from a deep link. */
export type GitHubRepoRef = {
  branch?: string;
  owner: string;
  /** Directory the link pointed at, without leading or trailing slashes. */
  path?: string;
  repo: string;
};

export type GitHubRepository = {
  branches: string[];
  defaultBranch: string;
};

/** One node of the repository's tree, as the dialog needs it. */
export type GitHubTree = {
  /** Every directory, "/" first, then the rest depth-first by path. */
  directories: string[];
  /** Candidate contract files per directory, best match first. */
  filesByDirectory: Record<string, string[]>;
  /** GitHub gave up part-way: a very large repository is only partly listed. */
  truncated: boolean;
};

type Failure =
  | { status: 'notFound' }
  /** 60 anonymous requests an hour, and they are spent. */
  | { status: 'rateLimited' }
  | { status: 'failed' };

export type GitHubRepositoryResult = { repository: GitHubRepository; status: 'found' } | Failure;

export type GitHubTreeResult = { status: 'loaded'; tree: GitHubTree } | Failure;

const asRecord = (value: unknown): Record<string, unknown> | null =>
  typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;

/** GitHub answers a spent quota with 403 and a zeroed remaining-count header. */
const failureFor = (response: Response): Failure => {
  if (response.status === 404) {
    return { status: 'notFound' };
  }
  if (
    (response.status === 403 || response.status === 429) &&
    response.headers.get('x-ratelimit-remaining') === '0'
  ) {
    return { status: 'rateLimited' };
  }
  return { status: 'failed' };
};

const readJson = async (
  url: string,
  signal?: AbortSignal,
): Promise<{ payload: unknown } | Failure> => {
  try {
    const response = await fetch(url, {
      headers: { Accept: 'application/vnd.github+json' },
      signal,
    });
    if (!response.ok) {
      return failureFor(response);
    }
    return { payload: await response.json() };
  } catch {
    // Network failure, an abort, or a body that wasn't JSON. Developer-facing,
    // so the reason stays out of the UI.
    return { status: 'failed' };
  }
};

/**
 * A GitHub repository URL, rather than any URL that mentions GitHub: the host
 * must be github.com itself and the path must name both owner and repository.
 */
export const isGitHubRepositoryUrl = (value: string): boolean => {
  if (!isValidUrl(value)) {
    return false;
  }
  try {
    const url = new URL(value);
    if (url.hostname !== 'github.com' && url.hostname !== 'www.github.com') {
      return false;
    }
    return url.pathname.split('/').filter((segment) => segment !== '').length >= 2;
  } catch {
    return false;
  }
};

/**
 * Reads owner, repository and — from a deep link — branch and directory out of
 * a GitHub URL.
 *
 * Accepts the plain `github.com/owner/repo` form, the `.git` clone form, and
 * `github.com/owner/repo/tree/<branch>/<path>` as pasted from the browser's
 * address bar. Returns `null` for anything that isn't github.com.
 */
export const parseGitHubRepoUrl = (value: string): GitHubRepoRef | null => {
  let url: URL;
  try {
    url = new URL(value.trim());
  } catch {
    return null;
  }
  if (url.hostname !== 'github.com' && url.hostname !== 'www.github.com') {
    return null;
  }

  const segments = url.pathname.split('/').filter((segment) => segment !== '');
  const [owner, rawRepo, kind, branch, ...rest] = segments;
  if (owner === undefined || rawRepo === undefined) {
    return null;
  }

  const repo = rawRepo.replace(/\.git$/, '');
  // Only `tree` carries a branch and directory; `blob`, `commit` and the rest
  // name something this step can't start from, so they are ignored.
  if (kind !== 'tree' || branch === undefined) {
    return { owner, repo };
  }
  return {
    branch,
    owner,
    path: rest.join('/'),
    repo,
  };
};

/** The repository itself, plus every branch it publishes. */
export const lookupGitHubRepository = async (
  ref: GitHubRepoRef,
  signal?: AbortSignal,
): Promise<GitHubRepositoryResult> => {
  const repoUrl = `${API_BASE_URL}/repos/${encodeURIComponent(
    ref.owner,
  )}/${encodeURIComponent(ref.repo)}`;

  const repoResult = await readJson(repoUrl, signal);
  if (!('payload' in repoResult)) {
    return repoResult;
  }
  const defaultBranch = asRecord(repoResult.payload)?.default_branch;
  if (typeof defaultBranch !== 'string' || defaultBranch === '') {
    return { status: 'failed' };
  }

  const branchesResult = await readJson(`${repoUrl}/branches?per_page=100`, signal);
  const branches = Array.isArray('payload' in branchesResult ? branchesResult.payload : null)
    ? (branchesResult as { payload: unknown[] }).payload
        .map((entry) => asRecord(entry)?.name)
        .filter((name): name is string => typeof name === 'string')
    : [];

  return {
    repository: {
      // The default branch always belongs in the list, even when the branch
      // listing failed or paged past it.
      branches: branches.includes(defaultBranch) ? branches : [defaultBranch, ...branches],
      defaultBranch,
    },
    status: 'found',
  };
};

/** Files this step could import: an OpenAPI document is YAML or JSON. */
const CONTRACT_FILE_PATTERN = /\.(ya?ml|json)$/i;

/** Names that usually mean "this is the contract", best first. */
const PREFERRED_NAMES = [/^openapi\./i, /^swagger\./i, /^api\./i];

const fileRank = (name: string): number => {
  const match = PREFERRED_NAMES.findIndex((pattern) => pattern.test(name));
  return match === -1 ? PREFERRED_NAMES.length : match;
};

/** Sorts by how much the name promises, then alphabetically. */
const byPreference = (left: string, right: string): number =>
  fileRank(left) - fileRank(right) || left.localeCompare(right);

/** The directory a path sits in, as the tree keys them: "/" or "/apis/orders". */
const directoryOf = (path: string): string => {
  const cut = path.lastIndexOf('/');
  return cut === -1 ? '/' : `/${path.slice(0, cut)}`;
};

/**
 * The repository's whole tree for one branch, in a single request.
 *
 * `recursive=1` is what makes one call enough — GitHub truncates the answer for
 * very large repositories, which `truncated` reports so the dialog can say the
 * listing is partial rather than pretend a folder is missing.
 */
export const fetchGitHubTree = async (
  owner: string,
  repo: string,
  branch: string,
  signal?: AbortSignal,
): Promise<GitHubTreeResult> => {
  const result = await readJson(
    `${API_BASE_URL}/repos/${encodeURIComponent(owner)}/${encodeURIComponent(
      repo,
    )}/git/trees/${encodeURIComponent(branch)}?recursive=1`,
    signal,
  );
  if (!('payload' in result)) {
    return result;
  }

  const payload = asRecord(result.payload);
  const entries = payload?.tree;
  if (!Array.isArray(entries)) {
    return { status: 'failed' };
  }

  const directories = new Set<string>(['/']);
  const filesByDirectory: Record<string, string[]> = {};

  for (const entry of entries) {
    const node = asRecord(entry);
    const path = node?.path;
    if (typeof path !== 'string' || path === '') {
      continue;
    }
    if (node?.type === 'tree') {
      directories.add(`/${path}`);
      continue;
    }
    if (node?.type !== 'blob' || !CONTRACT_FILE_PATTERN.test(path)) {
      continue;
    }
    const directory = directoryOf(path);
    const name = path.slice(path.lastIndexOf('/') + 1);
    filesByDirectory[directory] = [...(filesByDirectory[directory] ?? []), name];
  }

  for (const files of Object.values(filesByDirectory)) {
    files.sort(byPreference);
  }

  return {
    status: 'loaded',
    tree: {
      directories: [
        '/',
        ...[...directories].filter((path) => path !== '/').sort((a, b) => a.localeCompare(b)),
      ],
      filesByDirectory,
      truncated: payload?.truncated === true,
    },
  };
};

/** Candidate contract files in one directory, best match first. */
export const contractFilesIn = (tree: GitHubTree | null, directory: string): string[] =>
  tree?.filesByDirectory[directory] ?? [];

/** Where one file's bytes live. Raw content is served cross-origin. */
export const rawFileUrl = (
  owner: string,
  repo: string,
  branch: string,
  directory: string,
  file: string,
): string => {
  const segments = [...directory.split('/'), file]
    .filter((segment) => segment !== '')
    .map(encodeURIComponent);
  return `${RAW_BASE_URL}/${encodeURIComponent(owner)}/${encodeURIComponent(
    repo,
  )}/${encodeURIComponent(branch)}/${segments.join('/')}`;
};
