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
 * A build id shortened for display. Build ids are UUIDs, which are unreadable in
 * full but recognisable by their first segment — enough to tell two builds apart
 * on screen, while the full id stays available wherever it is acted on.
 */
export function shortBuild(buildId?: string): string {
  if (!buildId) return '—';
  return `Build ${buildId.slice(0, 8)}`;
}
