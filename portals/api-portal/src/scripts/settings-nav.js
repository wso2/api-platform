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
 

// Panel navigation
(function () {
  var PANELS   = ['cfg-organizations', 'cfg-views', 'cfg-labels', 'cfg-plans', 'cfg-keymanagers', 'cfg-apis', 'cfg-mcps', 'cfg-webhooks', 'cfg-llm', 'cfg-workflows', 'cfg-theming'];
  var navItems = document.querySelectorAll('.cfg-nav-item');
  var panels   = document.querySelectorAll('.cfg-panel');

  function activate(id) {
    if (!PANELS.includes(id)) id = 'cfg-organizations';
    // The shared API/MCP wizard lives inside #cfg-apis. Any sidebar navigation exits it
    // and restores the APIs list, so we never land on a panel with the wizard still open.
    var wizard = document.getElementById('cfg-apis-wizard');
    if (wizard) wizard.style.display = 'none';
    var apisList = document.getElementById('cfg-apis-list');
    if (apisList) apisList.style.display = '';
    navItems.forEach(function(a) { a.classList.toggle('active', a.dataset.panel === id); });
    panels.forEach(function(p)   { p.style.display = p.id === id ? 'flex' : 'none'; });
    history.replaceState(null, '', location.pathname + '#' + id);
  }
  // Exposed so the API/MCP wizard (defined in the Manage-APIs script) can restore the
  // originating tab when it closes.
  window.cfgActivatePanel = activate;

  navItems.forEach(function(a) {
    a.addEventListener('click', function(e) { e.preventDefault(); activate(a.dataset.panel); });
  });

  var hash = location.hash.replace('#', '');
  var legacyMap = { 'apiworkflows': 'cfg-workflows', 'llm-instruction': 'cfg-llm' };
  activate(legacyMap[hash] || hash);
}());

// View switcher
(function () {
  document.querySelectorAll('.cfg-view-combo').forEach(function (combo) {
    var trigger = combo.querySelector('.cfg-view-combo-trigger');
    var menu    = combo.querySelector('.cfg-view-combo-menu');
    if (!trigger || !menu) return; // single-view org: trigger is a disabled display-only pill
    var input   = combo.querySelector('.cfg-view-combo-input');
    var empty   = combo.querySelector('.cfg-view-combo-empty');
    var options = Array.prototype.slice.call(combo.querySelectorAll('.cfg-view-combo-option'));

    function visibleOptions() { return options.filter(function (o) { return o.style.display !== 'none'; }); }
    function filter(q) {
      q = (q || '').toLowerCase().trim();
      var any = false;
      options.forEach(function (o) {
        var hit = o.dataset.name.toLowerCase().indexOf(q) !== -1 || o.dataset.value.toLowerCase().indexOf(q) !== -1;
        o.style.display = hit ? '' : 'none';
        if (hit) any = true;
      });
      if (empty) empty.hidden = any;
    }
    function open() {
      menu.hidden = false;
      trigger.setAttribute('aria-expanded', 'true');
      input.value = ''; filter('');
      input.focus();
    }
    function close() {
      menu.hidden = true;
      trigger.setAttribute('aria-expanded', 'false');
    }

    trigger.addEventListener('click', function (e) {
      e.stopPropagation();
      if (menu.hidden) open(); else close();
    });
    input.addEventListener('input', function () { filter(this.value); });
    input.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') { close(); trigger.focus(); }
      else if (e.key === 'Enter') {
        var first = visibleOptions()[0];
        if (first) { e.preventDefault(); window.location.href = first.getAttribute('href'); }
      } else if (e.key === 'ArrowDown') {
        e.preventDefault(); var v = visibleOptions(); if (v[0]) v[0].focus();
      }
    });
    options.forEach(function (o) {
      o.addEventListener('keydown', function (e) {
        var v = visibleOptions(); var i = v.indexOf(o);
        if (e.key === 'ArrowDown') { e.preventDefault(); (v[i + 1] || v[0]).focus(); }
        else if (e.key === 'ArrowUp') { e.preventDefault(); if (i <= 0) input.focus(); else v[i - 1].focus(); }
        else if (e.key === 'Escape') { close(); trigger.focus(); }
      });
    });
    document.addEventListener('click', function (e) { if (!combo.contains(e.target)) close(); });
  });
}());
