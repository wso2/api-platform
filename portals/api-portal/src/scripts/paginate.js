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

/*
 * Client-side pagination shared by the public API/MCP card listings and the
 * Settings APIs/MCPs tables. Every item is already rendered server-side (the
 * portal loads the full catalog and search is a server round-trip), so this
 * only toggles per-page visibility and draws a control bar — no data fetching.
 * Below its page-size threshold a surface shows everything and no controls.
 */
(function () {
  'use strict';

  var GRID_PAGE_SIZE = 12; // multiple of 3 — fits the 3-column card grid
  var TABLE_PAGE_SIZE = 10;

  function el(tag, cls) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    return e;
  }

  // Compact page list around the current page: 1 … c-1 c c+1 … N
  function pageNumbers(pages, current) {
    var out = [];
    for (var p = 1; p <= pages; p++) {
      if (p === 1 || p === pages || (p >= current - 1 && p <= current + 1)) {
        out.push(p);
      } else if (out[out.length - 1] !== '…') {
        out.push('…');
      }
    }
    return out;
  }

  /*
   * getItems() is called on every apply() so row removals (bulk/single delete in
   * Settings) are picked up without the caller re-wiring anything. `mount` is the
   * element the control bar renders into; `anchor` (optional) is scrolled to the
   * top on a page change so the user isn't left mid-list.
   */
  function createPaginator(getItems, pageSize, mount, anchor) {
    var current = 1;

    function go(p) {
      current = p;
      apply();
      if (anchor && anchor.scrollIntoView) {
        anchor.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
    }

    function apply() {
      var items = getItems();
      var pages = Math.max(1, Math.ceil(items.length / pageSize));
      if (current > pages) current = pages;
      if (current < 1) current = 1;

      // Not enough items to paginate — show them all, hide the controls.
      if (items.length <= pageSize) {
        items.forEach(function (it) { it.style.display = ''; });
        mount.innerHTML = '';
        mount.style.display = 'none';
        return;
      }

      var start = (current - 1) * pageSize;
      var end = Math.min(start + pageSize, items.length);
      items.forEach(function (it, i) {
        it.style.display = (i >= start && i < end) ? '' : 'none';
      });
      mount.style.display = '';
      renderControls(pages, start, end, items.length);
    }

    function navBtn(cls, iconClass, label, disabled, onClick) {
      var b = el('button', 'dp-page-btn ' + cls);
      b.type = 'button';
      b.setAttribute('aria-label', label);
      b.innerHTML = '<i class="bi ' + iconClass + '"></i>';
      b.disabled = disabled;
      if (!disabled) b.addEventListener('click', onClick);
      return b;
    }

    function renderControls(pages, start, end, total) {
      mount.innerHTML = '';

      var info = el('span', 'dp-pagination-info');
      info.textContent = 'Showing ' + (start + 1) + '–' + end + ' of ' + total;
      mount.appendChild(info);

      var nav = el('div', 'dp-pagination-nav');
      nav.appendChild(navBtn('dp-page-nav', 'bi-chevron-left', 'Previous page',
        current === 1, function () { go(current - 1); }));

      pageNumbers(pages, current).forEach(function (p) {
        if (p === '…') {
          var gap = el('span', 'dp-page-ellipsis');
          gap.textContent = '…';
          nav.appendChild(gap);
          return;
        }
        var b = el('button', 'dp-page-btn' + (p === current ? ' dp-page-btn--active' : ''));
        b.type = 'button';
        b.textContent = String(p);
        if (p === current) b.setAttribute('aria-current', 'page');
        b.addEventListener('click', function () { go(p); });
        nav.appendChild(b);
      });

      nav.appendChild(navBtn('dp-page-nav', 'bi-chevron-right', 'Next page',
        current === pages, function () { go(current + 1); }));

      mount.appendChild(nav);
    }

    return { apply: apply };
  }

  // Public card grids (/apis and /mcps) — items are the .col-* card wrappers.
  function paginateGrid() {
    var grid = document.querySelector('.apilist-container .row.row-gap-4');
    if (!grid) return;
    var getItems = function () {
      return Array.prototype.filter.call(grid.children, function (c) {
        return c.nodeType === 1;
      });
    };
    if (getItems().length === 0) return; // empty-state markup, no grid children
    var mount = el('div', 'dp-pagination');
    grid.parentNode.insertBefore(mount, grid.nextSibling);
    createPaginator(getItems, GRID_PAGE_SIZE, mount, grid).apply();
  }

  // Settings tables — items are the API/MCP rows (the empty-state row has no id).
  function paginateTable(tbodyId) {
    var tbody = document.getElementById(tbodyId);
    if (!tbody) return;
    var wrap = tbody.closest('.cfg-table-wrap') || tbody.parentNode;
    var getItems = function () {
      return Array.prototype.slice.call(tbody.querySelectorAll('tr[id^="cfg-api-row-"]'));
    };
    var mount = el('div', 'dp-pagination');
    wrap.parentNode.insertBefore(mount, wrap.nextSibling);
    var paginator = createPaginator(getItems, TABLE_PAGE_SIZE, mount, wrap);
    paginator.apply();
    // Rows are removed on delete; reflow so the current page and controls stay
    // correct. childList-only means our own display toggles don't retrigger it.
    new MutationObserver(function () { paginator.apply(); })
      .observe(tbody, { childList: true });
  }

  document.addEventListener('DOMContentLoaded', function () {
    paginateGrid();
    paginateTable('cfg-apis-tbody');
    paginateTable('cfg-mcps-tbody');
  });
})();
