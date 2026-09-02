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

import { ColorSchemeSVG } from '@wso2/oxygen-ui';

/**
 * A gateway routing a client's calls out to a set of backends — the "here is
 * what this page holds once you create something" illustration for the
 * first-run `EmptyState` on the gateway listing, and the counterpart to
 * `MonitorIllustration` on the API listing and `ProjectFolderIllustration` on
 * the project listing.
 *
 * It draws what a gateway *is* rather than what it looks like in a rack: one
 * thing in the middle that every call passes through. That is the idea the
 * empty state is asking the user to create.
 *
 * Purely decorative: the `EmptyState` heading and body carry the meaning, so it
 * is hidden from assistive technology.
 */
export function GatewayIllustration() {
  return (
    <ColorSchemeSVG aria-hidden height={140} viewBox="0 0 220 140" width={220}>
      {/* The calling client, and its hop into the gateway. */}
      <rect fill="muted" height={32} rx={4} width={40} x={12} y={54} />
      <rect fill="background" height={5} rx={2} width={24} x={20} y={62} />
      <rect fill="background" height={5} rx={2} width={16} x={20} y={72} />
      <path d="M52 70h24" stroke="border" strokeLinecap="round" strokeWidth={2} />

      {/* The gateway itself: the one thing every call passes through. */}
      <rect fill="text-primary" height={84} rx={6} width={68} x={76} y={28} />
      <rect fill="primary" height={10} rx={2} width={10} x={88} y={38} />
      <rect fill="muted" height={5} rx={2} width={30} x={102} y={41} />

      {/* Its routing table, filled with the page background so it stays
          legible whichever way the slab above resolves. */}
      <rect fill="background" height={38} rx={3} width={48} x={86} y={58} />
      <rect fill="primary" height={5} rx={2} width={28} x={92} y={65} />
      <rect fill="muted" height={4} rx={2} width={36} x={92} y={76} />
      <rect fill="muted" height={4} rx={2} width={22} x={92} y={85} />

      {/* The fan out to the backends behind it. */}
      <path d="M144 70h10V44h14" fill="none" stroke="border" strokeWidth={2} />
      <path d="M144 70h24" stroke="border" strokeLinecap="round" strokeWidth={2} />
      <path d="M144 70h10V96h14" fill="none" stroke="border" strokeWidth={2} />

      {/* The backends it fronts. */}
      <rect fill="primary" height={18} rx={4} width={40} x={168} y={35} />
      <rect fill="primary" height={18} rx={4} width={40} x={168} y={61} />
      <rect fill="primary" height={18} rx={4} width={40} x={168} y={87} />

      {/* The surface it rests on. */}
      <path d="M50 132h120" stroke="border" strokeLinecap="round" strokeWidth={2} />
    </ColorSchemeSVG>
  );
}
