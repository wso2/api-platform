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

import Editor, { loader } from '@monaco-editor/react';
import { useTheme } from '@wso2/oxygen-ui';
// The package root, i.e. the whole editor. A hand-picked subset was measured
// and rejected: excluding the 81 language grammars and the CSS/HTML/TypeScript
// services saves 2.5% (4,019kB -> 3,919kB), because Rollup already code-splits
// those out on its own. What remains is Monaco's core and its 74 feature
// contributions, which is the editor itself and not reducible. The full import
// is also the only one the package supports across upgrades.
//
// Worker subpaths go through the package's `exports` map
// ("./*.js" -> "./esm/vs/*.js"), which is why they carry no `esm/vs` prefix.
// `?worker` is Vite's own suffix: it builds each as a separate worker bundle
// and hands back a constructor.
import * as monaco from 'monaco-editor';
import editorWorker from 'monaco-editor/editor/editor.worker.js?worker';
import jsonWorker from 'monaco-editor/languages/features/json/json.worker.js?worker';
import { useEffect, useState } from 'react';

import type { SpecFormat } from '../utils/specText';

/**
 * Monaco, wired to this app rather than to its own defaults.
 *
 * Everything that touches the `monaco-editor` package lives in this one module
 * so the rest of the step doesn't: it is the only file that has to change if
 * Monaco moves its ESM paths again, and — because it is loaded lazily by
 * `SpecSourceEditor` — the only one that drags the editor's weight into a
 * chunk. Tests replace this module wholesale rather than running Monaco in
 * jsdom, which it does not support.
 */

/**
 * Monaco is served from this bundle, never from a CDN.
 *
 * `@monaco-editor/loader` otherwise fetches the editor from jsdelivr at first
 * render, which would put a working Source view behind public network egress —
 * broken in an air-gapped install, and blocked by any reasonable
 * `script-src`. Handing it the imported instance keeps the editor local.
 */
loader.config({ monaco });

/**
 * Where Monaco's language services run.
 *
 * Without this the ESM build throws on first mount rather than degrading, so it
 * is set at module scope: by the time any editor renders, it is already there.
 * JSON gets its own worker — that is what makes completion, hover and
 * as-you-type diagnostics work. YAML has no worker in Monaco at all; it is
 * highlighted and folded from a grammar on the main thread, and the definition
 * it produces is still checked in full on Save, by the same
 * `validateApiSpec` a JSON one goes through.
 */
window.MonacoEnvironment = {
  getWorker: (_workerId: string, label: string) =>
    label === 'json' ? new jsonWorker() : new editorWorker(),
};

/** Monaco's own language ids for the two formats the source view offers. */
const LANGUAGE_FOR: Record<SpecFormat, string> = {
  json: 'json',
  yaml: 'yaml',
};

/**
 * Which of Monaco's built-in themes matches the app's.
 *
 * Read the same way `CodeBlock` reads it — the `data-color-scheme` attribute
 * Oxygen stamps on the document element, falling back to the palette — so a
 * highlighted block and the editor below it are never in opposite schemes.
 */
const useIsDarkScheme = (): boolean => {
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

export type SpecCodeEditorProps = {
  /** Which language Monaco highlights and completes against. */
  format: SpecFormat;
  /**
   * Room for a minimap. Off in the pane, where it would eat a third of an
   * already narrow column; on in the expanded view, where it earns its place.
   */
  minimap?: boolean;
  /** Every keystroke, with the buffer's full text. Absent while read-only. */
  onChange?: (value: string) => void;
  /** Read mode. The editor still scrolls, folds and selects — it just can't be typed into. */
  readOnly?: boolean;
  /** The text to show. */
  value: string;
};

/**
 * A code editor sized to fill whatever it is put in.
 *
 * The height comes from the parent rather than a prop: the same editor has to
 * fill a column of the contract step and, unchanged, a full-height side panel.
 */
export const SpecCodeEditor = ({
  format,
  minimap = false,
  onChange,
  readOnly = false,
  value,
}: SpecCodeEditorProps) => {
  const isDark = useIsDarkScheme();

  return (
    <Editor
      height="100%"
      language={LANGUAGE_FOR[format]}
      onChange={(next) => onChange?.(next ?? '')}
      options={{
        // The pane and the panel are both resizable; without this the editor
        // keeps whatever size it was first measured at.
        automaticLayout: true,
        fontSize: 13,
        // A definition is mostly long URLs and descriptions, and this editor is
        // often in a narrow column — wrapping beats a horizontal scrollbar.
        wordWrap: 'on',
        lineNumbers: 'on',
        minimap: { enabled: minimap },
        readOnly,
        renderLineHighlight: readOnly ? 'none' : 'line',
        scrollBeyondLastLine: false,
        tabSize: 2,
      }}
      theme={isDark ? 'vs-dark' : 'light'}
      value={value}
    />
  );
};

export default SpecCodeEditor;
