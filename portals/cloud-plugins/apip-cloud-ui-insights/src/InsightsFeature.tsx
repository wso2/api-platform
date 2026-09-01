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

  const scopeLevel =
    forcedScopeLevel ??
    resolveInsightsScopeLevel({
      projectHandle,
    });

  const [projectId, setProjectId] = useState<string | null>(null);
  const [projectName, setProjectName] = useState<string | null>(null);
  const [scopeError, setScopeError] = useState<string | null>(null);
  const [scopeLoading, setScopeLoading] = useState(scopeLevel === 'project');

  const scopeKey = useMemo(
    () => [orgHandle, projectHandle ?? '', scopeLevel].join('|'),
    [orgHandle, projectHandle, scopeLevel]
  );

  useEffect(() => {
    if (scopeLevel !== 'project') {
      setScopeLoading(false);
      setScopeError(null);
      setProjectId(null);
      setProjectName(null);
      return;
    }

    let cancelled = false;
    setScopeLoading(true);
    setScopeError(null);
    setProjectId(null);
    setProjectName(null);

    (async () => {
      try {
        if (!orgHandle) {
          throw new Error('Organization context is unavailable.');
        }
        if (!projectHandle) {
          throw new Error('Project scope is required for project insights.');
        }
        const project = await resolveProjectScope(orgHandle, projectHandle);
        if (cancelled) return;
        setProjectId(project.projectId);
        setProjectName(project.projectName);
      } catch (err: unknown) {
        if (!cancelled) {
          setScopeError(err instanceof Error ? err.message : String(err));
        }
      } finally {
        if (!cancelled) setScopeLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [orgHandle, projectHandle, scopeKey, scopeLevel]);

  if (!orgHandle) {
    return (
      <ErrorState
        message="Organization context is unavailable."
        title="Unable to load Insights"
      />
    );
  }

  if (scopeLoading) {
    return <LoadingState label="Preparing Insights" />;
  }

  if (scopeError) {
    return (
      <ErrorState message={scopeError} title="Unable to load Insights" />
    );
  }

  return (
    <InsightsEmbed
      scope={{
        level: scopeLevel,
        projectId,
        projectName,
      }}
    />
  );
};

export default InsightsFeature;
