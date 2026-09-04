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

import { useEffect, useMemo, useState, type FC } from 'react';

import { resolveProjectScope } from './api/analyticsApi';
import { ErrorState, LoadingState } from './components/StateViews';
import type { InsightsHostPort } from './hostPort';
import InsightsEmbed from './InsightsEmbed';
import type { InsightsScopeLevel } from './types';
import { resolveInsightsScopeLevel } from './utils/moesifEmbed';
import { parseInsightsRouteParams } from './utils/routeParams';

export type InsightsFeatureProps = {
  port: InsightsHostPort;
  /** When set, overrides URL-derived scope. */
  forcedScopeLevel?: InsightsScopeLevel;
};

/**
 * Resolves project scope when needed, then renders the wrap/basic Moesif embed.
 *
 * Until Moesif reliably supports project filtering, a failed project resolve
 * falls back to the organization embed instead of blocking the page.
 *
 * Organization context for the viewer token comes from the BFF session — the
 * embed does not need `idpOrganizationRefUuid` from the platform-api org list.
 */
const InsightsFeature: FC<InsightsFeatureProps> = ({
  port,
  forcedScopeLevel,
}) => {
  const routeParams = parseInsightsRouteParams(window.location.pathname);
  const orgHandle = port.orgHandle || routeParams.orgHandle || '';
  const projectHandle = port.projectHandle || routeParams.projectHandler;

  const requestedScopeLevel =
    forcedScopeLevel ??
    resolveInsightsScopeLevel({
      projectHandle,
    });

  const scopeKey = useMemo(
    () => [orgHandle, projectHandle ?? '', requestedScopeLevel].join('|'),
    [orgHandle, projectHandle, requestedScopeLevel]
  );

  const [embedScopeLevel, setEmbedScopeLevel] =
    useState<InsightsScopeLevel>(requestedScopeLevel);
  const [projectId, setProjectId] = useState<string | null>(null);
  const [projectName, setProjectName] = useState<string | null>(null);
  const [scopeError, setScopeError] = useState<string | null>(null);
  const [scopeLoading, setScopeLoading] = useState(
    requestedScopeLevel === 'project'
  );
  const [resolvedScopeKey, setResolvedScopeKey] = useState<string | null>(() =>
    requestedScopeLevel === 'project' ? null : scopeKey
  );

  const isScopeReady = resolvedScopeKey === scopeKey;

  useEffect(() => {
    if (requestedScopeLevel !== 'project') {
      setEmbedScopeLevel('organization');
      setScopeLoading(false);
      setScopeError(null);
      setProjectId(null);
      setProjectName(null);
      setResolvedScopeKey(scopeKey);
      return;
    }

    let cancelled = false;
    setScopeLoading(true);
    setScopeError(null);
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
          // No project context — use org embed until Moesif project support is ready.
          if (!cancelled) {
            setEmbedScopeLevel('organization');
            setProjectId(null);
            setProjectName(null);
            setResolvedScopeKey(scopeKey);
          }
          return;
        }
        const project = await resolveProjectScope(orgHandle, projectHandle);
        if (cancelled) return;
        setEmbedScopeLevel('project');
        setProjectId(project.projectId);
        setProjectName(project.projectName);
        setResolvedScopeKey(scopeKey);
      } catch {
        // Project-level Insights unavailable (resolve failed / Moesif not ready) —
        // fall back to organization wrap/basic.
        if (!cancelled) {
          setEmbedScopeLevel('organization');
          setProjectId(null);
          setProjectName(null);
          setScopeError(null);
          setResolvedScopeKey(scopeKey);
        }
      } finally {
        if (!cancelled) setScopeLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [orgHandle, projectHandle, scopeKey, requestedScopeLevel]);

  if (!orgHandle) {
    return (
      <ErrorState
        message="Organization context is unavailable."
        title="Unable to load Insights"
      />
    );
  }

  if (scopeError) {
    return (
      <ErrorState message={scopeError} title="Unable to load Insights" />
    );
  }

  if (scopeLoading || !isScopeReady) {
    return <LoadingState label="Preparing Insights" />;
  }

  return (
    <InsightsEmbed
      scope={{
        level: embedScopeLevel,
        projectId,
        projectName,
      }}
    />
  );
};

export default InsightsFeature;
