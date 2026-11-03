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
import { useTheme } from '@wso2/oxygen-ui';
import { useEffect, useState } from 'react';

/**
 * Whether the app is currently in its dark colour scheme.
 *
 * Read from the `data-color-scheme` attribute Oxygen stamps on the document
 * element (the same signal `CodeBlock` uses), falling back to the palette, so
 * every embedded third-party editor follows the app's scheme rather than the
 * operating system's.
 */
export const useIsDarkScheme = (): boolean => {
  const theme = useTheme();
  const [isDark, setIsDark] = useState(false);

  useEffect(() => {
    const read = () => {
      const scheme = document.documentElement.getAttribute('data-color-scheme');
      setIsDark(scheme === 'dark' || theme.palette.mode === 'dark');
    };
    read();

    // The attribute is set outside React, so an observer is what notices it.
    const observer = new MutationObserver(read);
    observer.observe(document.documentElement, {
      attributeFilter: ['data-color-scheme'],
      attributes: true,
    });
    return () => observer.disconnect();
  }, [theme.palette.mode]);

  return isDark;
};
