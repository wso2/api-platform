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
 * A small stack of an API's documents — a guide open on top of the others,
 * with the type tabs they are grouped under — for the first-run `EmptyState`
 * on Develop › Documents. Drawn in the same shapes and colour roles as
 * `GatewayIllustration`, `MonitorIllustration` and `ProjectFolderIllustration`
 * so every empty page in the console reads as one family.
 *
 * Purely decorative: the `EmptyState` heading carries the meaning, so it is
 * hidden from assistive technology.
 */
export function DocumentsIllustration() {
  return (
    <ColorSchemeSVG aria-hidden height={140} viewBox="0 0 220 140" width={220}>
      {/* The documents behind, fanned out. */}
      <rect fill="muted" height={84} rx={6} width={64} x={58} y={30} />
      <rect fill="muted" height={84} rx={6} width={64} x={70} y={24} />

      {/* The open guide: a dark slab with its page filled with the page
          background, so it stays legible whichever way the slab resolves. */}
      <rect fill="text-primary" height={92} rx={6} width={72} x={84} y={18} />
      <rect fill="primary" height={10} rx={2} width={10} x={94} y={28} />
      <rect fill="muted" height={5} rx={2} width={34} x={108} y={31} />
      <rect fill="background" height={56} rx={3} width={56} x={92} y={46} />
      <rect fill="primary" height={5} rx={2} width={30} x={98} y={53} />
      <rect fill="muted" height={4} rx={2} width={44} x={98} y={64} />
      <rect fill="muted" height={4} rx={2} width={38} x={98} y={73} />
      <rect fill="muted" height={4} rx={2} width={42} x={98} y={82} />
      <rect fill="muted" height={4} rx={2} width={24} x={98} y={91} />

      {/* The type tabs the documents are filed under. */}
      <path d="M156 44h14" stroke="primary" strokeLinecap="round" strokeWidth={2} />
      <rect fill="primary" height={16} rx={4} width={36} x={170} y={36} />
      <path d="M156 66h14" stroke="primary" strokeLinecap="round" strokeWidth={2} />
      <rect fill="primary" height={16} rx={4} width={36} x={170} y={58} />

      {/* The surface it rests on. */}
      <path d="M50 132h120" stroke="border" strokeLinecap="round" strokeWidth={2} />
    </ColorSchemeSVG>
  );
}
