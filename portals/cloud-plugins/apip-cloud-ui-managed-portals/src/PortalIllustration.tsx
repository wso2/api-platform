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

import { Box } from '@wso2/oxygen-ui';

/** Decorative developer portal with API discovery cards and a code preview. */
export function PortalIllustration() {
  return (
    <Box
      component="svg"
      aria-hidden="true"
      focusable="false"
      height={140}
      viewBox="0 0 220 140"
      width={220}
      sx={{ maxWidth: '100%', height: 'auto' }}
    >
      {/* Browser window and portal navigation. */}
      <rect x={10} y={8} width={200} height={124} rx={8} fill="#242936" stroke="#485166" />
      <path d="M18 8h184a8 8 0 0 1 8 8v12H10V16a8 8 0 0 1 8-8Z" fill="#353D50" />
      <circle cx={21} cy={18} r={2.5} fill="#FF8055" />
      <circle cx={30} cy={18} r={2.5} fill="#F6C76A" />
      <circle cx={39} cy={18} r={2.5} fill="#64D8B1" />
      <rect x={66} y={15} width={90} height={6} rx={3} fill="#515C73" />
      <circle cx={27} cy={41} r={7} fill="#FF8055" />
      <path d="m23 41 3-3-1 6 3-3h3" fill="none" stroke="#242936" strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round" />
      <rect x={40} y={37} width={52} height={4} rx={2} fill="#F0F3FA" />
      <rect x={40} y={44} width={35} height={3} rx={1.5} fill="#929EB5" />
      <rect x={168} y={36} width={29} height={10} rx={4} fill="#FF8055" />

      {/* Search field above a colorful API catalog. */}
      <rect x={20} y={56} width={112} height={13} rx={4} fill="#353D50" stroke="#515C73" />
      <circle cx={28} cy={62} r={2.5} fill="none" stroke="#ACB8CC" strokeWidth={1.3} />
      <path d="m30 64 2 2" stroke="#ACB8CC" strokeWidth={1.3} strokeLinecap="round" />
      <rect x={39} y={61} width={57} height={3} rx={1.5} fill="#929EB5" />
      {[
        { x: 20, color: '#FF8055' },
        { x: 59, color: '#64D8B1' },
        { x: 98, color: '#A99BFF' },
      ].map(({ x, color }) => (
        <g key={x}>
          <rect x={x} y={77} width={34} height={43} rx={4} fill="#353D50" />
          <rect x={x + 5} y={83} width={13} height={13} rx={4} fill={color} />
          <path d={`m${x + 10} 87-2 2.5 2 2.5m3-5 2 2.5-2 2.5`} fill="none" stroke="#242936" strokeWidth={1.2} strokeLinecap="round" strokeLinejoin="round" />
          <rect x={x + 5} y={101} width={23} height={3} rx={1.5} fill="#E4E9F2" />
          <rect x={x + 5} y={108} width={17} height={3} rx={1.5} fill="#929EB5" />
        </g>
      ))}

      {/* Developer documentation and an API request example. */}
      <rect x={143} y={56} width={54} height={64} rx={4} fill="#18202D" />
      <rect x={149} y={63} width={26} height={4} rx={2} fill="#7CBFFF" />
      <path d="m153 76-4 4 4 4m31-8 4 4-4 4m-13-10-4 12" fill="none" stroke="#64D8B1" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
      <rect x={149} y={94} width={20} height={3} rx={1.5} fill="#A99BFF" />
      <rect x={173} y={94} width={17} height={3} rx={1.5} fill="#F6C76A" />
      <rect x={149} y={101} width={34} height={3} rx={1.5} fill="#929EB5" />
      <rect x={149} y={108} width={24} height={3} rx={1.5} fill="#7CBFFF" />
    </Box>
  );
}
