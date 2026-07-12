/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
/* eslint-disable no-undef */

(function() {
  // Copy an MCP server-config block to the clipboard (invoked via onclick in the sidebar).
  function copyMcpConfig(btn, id) {
    var el = document.getElementById('mcp-cfg-' + id);
    if (!el) return;
    var text = el.textContent || el.innerText || '';
    try { navigator.clipboard.writeText(text).catch(function(){}); } catch(e) {}
    btn.classList.add('copy-btn--copied');
    if (btn._copyTimer) clearTimeout(btn._copyTimer);
    btn._copyTimer = setTimeout(function() { btn.classList.remove('copy-btn--copied'); }, 1600);
  }
  window.copyMcpConfig = copyMcpConfig;

  // Syntax-highlight MCP JSON/config code blocks (highlight.js CDN loads before this defer script).
  document.querySelectorAll('.mc-json-block pre code, .mc-config-pre code').forEach(function(el) {
    hljs.highlightElement(el);
  });
}());
