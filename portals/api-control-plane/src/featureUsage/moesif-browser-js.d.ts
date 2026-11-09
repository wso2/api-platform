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
 * The part of moesif-browser-js (which ships no types) this app uses. Checked
 * against the package source (src/moesif.js), v1.10.2.
 */
declare module 'moesif-browser-js' {
  export type MoesifMetadata = Record<string, string | number | boolean>;

  export interface MoesifInitOptions {
    /** The Moesif application's publishable (collector) Application ID. */
    applicationId: string;
    batchEnabled?: boolean;
    batchSize?: number;
    batchMaxTime?: number;
    /** Return true to drop a captured API call. Only consulted after start(). */
    skip?: (event: unknown) => boolean;
    disableReferrer?: boolean;
    disableUtm?: boolean;
    disableGclid?: boolean;
  }

  export interface MoesifClient {
    init(options: MoesifInitOptions): MoesifClient;
    identifyUser(userId: string, metadata?: MoesifMetadata): void;
    identifyCompany(companyId: string, metadata?: MoesifMetadata, companyDomain?: string): void;
    track(actionName: string, metadata?: MoesifMetadata): void;
    /** Clears the stored user, company, session and queued requests. */
    reset(): void;
  }

  const moesif: MoesifClient;
  export default moesif;
}
