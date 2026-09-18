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

import { PageContent } from '@wso2/oxygen-ui';
import { useEffect, useMemo, useState, type FC, type ReactNode } from 'react';

import { resolveProjectScope } from './api/analyticsApi';
import { ErrorState, LoadingState } from './components/StateViews';
import { ENABLE_ACP_PROJECT_FILTER } from './config/acpProjectFilter';
import type { InsightsHostPort } from './hostPort';
import InsightsEmbed from './InsightsEmbed';
import type { InsightsEmbedProfile, InsightsScopeLevel } from './types';
import { resolveInsightsScopeLevel } from './utils/moesifEmbed';
import { parseInsightsRouteParams } from './utils/routeParams';

export { ENABLE_ACP_PROJECT_FILTER } from './config/acpProjectFilter';

/**
 * AI Workspace pages own their PageContent (the shell does not wrap the outlet).
 * ACP already wraps the outlet in PageContent — do not nest another here.
 */
function withHostPageChrome(
  embedProfile: InsightsEmbedProfile,
  children: ReactNode
): ReactNode {
  if (embedProfile === 'ai-workspace') {
    return <PageContent fullWidth>{children}</PageContent>;
  }
  return children;
}

export type InsightsFeatureProps = {
  port: InsightsHostPort;
  /** When set, overrides URL-derived scope. */
  forcedScopeLevel?: InsightsScopeLevel;
  /**
   * Host-chosen Moesif iframe path. AI Workspace uses the same ai-overview
   * URL at org and project (no project_id filtering).
   */
  embedProfile?: InsightsEmbedProfile;
};

/**
 * Resolves project scope when needed, then renders the wrap/basic Moesif embed.
 *
 * Organization context for the viewer token comes from the BFF session — the
 * embed does not need `idpOrganizationRefUuid` from the platform-api org list.
 */
const InsightsFeature: FC<InsightsFeatureProps> = ({
  port,
  forcedScopeLevel,
  embedProfile = 'api-control-plane',
}) => {
  const routeParams = parseInsightsRouteParams(window.location.pathname);
  const orgHandle = port.orgHandle || routeParams.orgHandle || '';
  const projectHandle = port.projectHandle || routeParams.projectHandler;

  const requestedScopeLevel =
    forcedScopeLevel ??
    resolveInsightsScopeLevel({
      projectHandle,
    });

  // AI Workspace: same iframe at org and project — skip project_id resolve.
  // ACP: gated by ENABLE_ACP_PROJECT_FILTER until Moesif project filter works.
  const needsProjectResolve =
    ENABLE_ACP_PROJECT_FILTER &&
    embedProfile === 'api-control-plane' &&
    requestedScopeLevel === 'project';

  const scopeKey = useMemo(
    () =>
      [orgHandle, projectHandle ?? '', requestedScopeLevel, embedProfile].join(
        '|'
      ),
    [embedProfile, orgHandle, projectHandle, requestedScopeLevel]
  );

  const [embedScopeLevel, setEmbedScopeLevel] =
    useState<InsightsScopeLevel>('organization');
  const [projectId, setProjectId] = useState<string | null>(null);
  const [projectName, setProjectName] = useState<string | null>(null);
  const [scopeLoading, setScopeLoading] = useState(needsProjectResolve);
  const [resolvedScopeKey, setResolvedScopeKey] = useState<string | null>(() =>
    needsProjectResolve ? null : scopeKey
  );

  const isScopeReady = resolvedScopeKey === scopeKey;

  useEffect(() => {
    if (!needsProjectResolve) {
      setEmbedScopeLevel('organization');
      setScopeLoading(false);
      setProjectId(null);
      setProjectName(null);
      setResolvedScopeKey(scopeKey);
      return;
    }

    let cancelled = false;
    setScopeLoading(true);
    setProjectId(null);
    setProjectName(null);
    setEmbedScopeLevel('project');
    setResolvedScopeKey(null);

    (async () => {
      try {
        if (!orgHandle) {
          throw new Error('Organization context is unavailable.');
        }
        if (!projectHandle) {
          if (!cancelled) {
            setEmbedScopeLevel('organization');
            setProjectId(null);
            setProjectName(null);
            setResolvedScopeKey(scopeKey);
          }
          return;
        }
        const project = await resolveProjectScope(port.apiFetch, projectHandle);
        if (cancelled) return;
        setEmbedScopeLevel('project');
        setProjectId(project.projectId);
        setProjectName(project.projectName);
        setResolvedScopeKey(scopeKey);
      } catch {
        // Project filter unavailable — keep Insights usable at org scope.
        if (!cancelled) {
          setEmbedScopeLevel('organization');
          setProjectId(null);
          setProjectName(null);
          setResolvedScopeKey(scopeKey);
        }
      } finally {
        if (!cancelled) setScopeLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [
    needsProjectResolve,
    orgHandle,
    port.apiFetch,
    projectHandle,
    scopeKey,
  ]);

  if (!orgHandle) {
    return withHostPageChrome(
      embedProfile,
      <ErrorState
        message="Organization context is unavailable."
        title="Unable to load Insights"
      />
    );
  }

  if (scopeLoading || !isScopeReady) {
    return withHostPageChrome(
      embedProfile,
      <LoadingState label="Preparing Insights" />
    );
  }

  return withHostPageChrome(
    embedProfile,
    <InsightsEmbed
      embedProfile={embedProfile}
      scope={{
        level: embedScopeLevel,
        projectId,
        projectName,
      }}
    />
  );
};

export default InsightsFeature;
