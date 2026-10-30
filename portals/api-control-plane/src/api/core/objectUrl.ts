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

import { useEffect, useState } from 'react';

/**
 * Shared, ref-counted `blob:` URLs keyed by Blob.
 *
 * Every consumer of the same Blob reuses one URL, and the URL is revoked only
 * once no mounted component holds it. The revoke is deferred so an immediate
 * remount (StrictMode's mount → cleanup → mount, or a quick route change between
 * pages that both show the image) re-acquires the same URL instead of rendering
 * a revoked one, which made the `<img>` fail until a full refresh.
 */
const objectUrls = new Map<Blob, { url: string; refs: number }>();

const objectUrlFor = (blob: Blob): string => {
  let entry = objectUrls.get(blob);
  if (!entry) {
    entry = { url: URL.createObjectURL(blob), refs: 0 };
    objectUrls.set(blob, entry);
  }
  return entry.url;
};

const retainObjectUrl = (blob: Blob): (() => void) => {
  objectUrlFor(blob);
  objectUrls.get(blob)!.refs += 1;
  return () => {
    const entry = objectUrls.get(blob);
    if (!entry) return;
    entry.refs -= 1;
    setTimeout(() => {
      const current = objectUrls.get(blob);
      if (current && current.refs <= 0) {
        URL.revokeObjectURL(current.url);
        objectUrls.delete(blob);
      }
    }, 0);
  };
};

/**
 * A `blob:` URL for `blob`, shared with every other holder and revoked once the last lets go.
 * `undefined` without a blob, and for the render before a newly given blob's URL is ready.
 * Made in the effect, not in render, so a discarded render can't leak an entry.
 */
export function useObjectUrl(blob: Blob | undefined): string | undefined {
  const [held, setHeld] = useState<{ blob: Blob; url: string }>();
  useEffect(() => {
    if (!blob) {
      setHeld(undefined);
      return;
    }
    setHeld({ blob, url: objectUrlFor(blob) });
    return retainObjectUrl(blob);
  }, [blob]);
  return held && held.blob === blob ? held.url : undefined;
}
