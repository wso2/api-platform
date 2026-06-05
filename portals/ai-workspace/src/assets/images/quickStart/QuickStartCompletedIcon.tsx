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

import React from 'react';

type QuickStartCompletedIconProps = React.SVGProps<SVGSVGElement> & {
  size?: number;
};

export default function QuickStartCompletedIcon({
  size = 16,
  ...props
}: QuickStartCompletedIconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      {...props}
    >
      <circle cx="8" cy="8" r="8" fill="#36B475" />
      <path
        d="M10.4848 5.73483C10.6313 5.58839 10.8687 5.58839 11.0152 5.73483C11.1483 5.86797 11.1604 6.0763 11.0515 6.22311L11.0152 6.26517L7.25 10.0303L4.98483 7.76517C4.83839 7.61872 4.83839 7.38128 4.98483 7.23483C5.11797 7.1017 5.3263 7.0896 5.47311 7.19853L5.51517 7.23483L7.25 8.9695L10.4848 5.73483Z"
        fill="white"
        stroke="white"
      />
    </svg>
  );
}
