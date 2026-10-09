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

/**
 * Turns a repository *page* link to a spec into the link to the file itself.
 *
 * The address bar on GitHub, GitLab or Bitbucket shows an HTML page that
 * renders the file, not the file. Pasted as a spec URL, the server downloads
 * that page and the import fails with a parser message the user can't act on.
 * Rewriting it is safe because the raw URL is the same file at a different
 * address; the caller writes the result back into the field and says so, so
 * the change is never silent.
 *
 * Only the hosted services are recognised. Self-hosted GitLab and anything
 * unrecognised pass through untouched (`undefined`).
 */

export type RepositoryHost = 'GitHub' | 'GitLab' | 'Bitbucket';

export type RawSpecUrl = { host: RepositoryHost; url: string };

export const rawSpecUrl = (input: string): RawSpecUrl | undefined => {
  let parsed: URL;
  try {
    parsed = new URL(input.trim());
  } catch {
    return undefined;
  }
  const host = parsed.hostname.toLowerCase();
  const tail = `${parsed.search}${parsed.hash}`;

  // github.com/{owner}/{repo}/blob/{ref…}/{path} → raw.githubusercontent.com/{owner}/{repo}/{ref…}/{path}
  if (host === 'github.com' || host === 'www.github.com') {
    const match = parsed.pathname.match(/^\/([^/]+)\/([^/]+)\/blob\/(.+)$/);
    if (!match) return undefined;
    return {
      host: 'GitHub',
      url: `https://raw.githubusercontent.com/${match[1]}/${match[2]}/${match[3]}${parsed.search}`,
    };
  }

  // gitlab.com/{group…}/{project}/-/blob/{ref}/{path} → …/-/raw/…
  if (host === 'gitlab.com' || host === 'www.gitlab.com') {
    if (!parsed.pathname.includes('/-/blob/')) return undefined;
    return {
      host: 'GitLab',
      url: `https://gitlab.com${parsed.pathname.replace('/-/blob/', '/-/raw/')}${tail}`,
    };
  }

  // bitbucket.org/{workspace}/{repo}/src/{ref}/{path} → …/raw/…
  if (host === 'bitbucket.org' || host === 'www.bitbucket.org') {
    const match = parsed.pathname.match(/^\/([^/]+)\/([^/]+)\/src\/(.+)$/);
    if (!match) return undefined;
    return {
      host: 'Bitbucket',
      url: `https://bitbucket.org/${match[1]}/${match[2]}/raw/${match[3]}${parsed.search}`,
    };
  }

  return undefined;
};
