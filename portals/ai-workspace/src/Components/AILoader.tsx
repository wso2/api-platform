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
type AILoaderProps = {
  label?: string;
};

export default function AILoader({ label }: AILoaderProps) {
  return (
    <div className="loader">
      <svg width="100" height="100" viewBox="0 0 100 100">
        <defs>
          <linearGradient
            id="loaderGradient"
            gradientUnits="userSpaceOnUse"
            x1="0"
            y1="0"
            x2="0"
            y2="100"
          >
            <stop offset="30%" stopColor="#f36822" />
            <stop offset="70%" stopColor="#e27a3f" />
          </linearGradient>
          <filter
            id="loaderGoo"
            filterUnits="userSpaceOnUse"
            x="-50"
            y="-50"
            width="200"
            height="200"
          >
            <feGaussianBlur in="SourceGraphic" stdDeviation="12" result="blur" />
            <feColorMatrix
              in="blur"
              mode="matrix"
              values="1 0 0 0 0
                      0 1 0 0 0
                      0 0 1 0 0
                      0 0 0 8 -0.5"
            />
          </filter>
        </defs>
        <g id="clipping" filter="url(#loaderGoo)">
          <polygon points="-50,-50 150,-50 150,150 -50,150" fill="none"></polygon>
          <polygon points="25,25 75,25 50,75" fill="url(#loaderGradient)"></polygon>
          <polygon points="50,25 75,75 25,75" fill="url(#loaderGradient)"></polygon>
          <polygon points="35,35 65,35 50,65" fill="url(#loaderGradient)"></polygon>
          <polygon points="35,35 65,35 50,65" fill="url(#loaderGradient)"></polygon>
          <polygon points="35,35 65,35 50,65" fill="url(#loaderGradient)"></polygon>
          <polygon points="35,35 65,35 50,65" fill="url(#loaderGradient)"></polygon>
        </g>
      </svg>
    </div>
  );
}
