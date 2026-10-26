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

'use strict';

/*
 * A text box that also offers a few suggestions in the view switcher's menu: a Tokens meter's event
 * key (settings-meters.js) and the API type values of a price filter (settings-plans.js). The list
 * always shows every suggestion, regardless of what is typed. Choosing one fills it in, or adds it to the
 * comma-separated values when the box takes several; any typed text is kept.
 *
 * The markup, which cfg-modals.hbs writes for the Token row and build() writes around an input:
 *   .cfg-key-combo
 *     input[role=combobox][aria-controls=<list id>]
 *     button.cfg-key-combo-toggle
 *     .cfg-view-combo-menu[hidden] > .cfg-view-combo-list[role=listbox]#<list id>
 *       [role=option][data-value][aria-label] per suggestion
 *
 * Keyboard: the toggle or the Down arrow opens the list on the value in the box (or the first
 * suggestion); the arrows move through it, Enter or Space chooses, Escape and Tab close it.
 *
 * Loaded as a plain script before its callers, like cfg-chip-input.js, so it is set on window.
 */
(function () {
  var listCount = 0;
  // At most one list is open; a click anywhere outside its combo closes it.
  var openList = null;

  document.addEventListener('click', function (e) {
    if (openList && !openList.combo.contains(e.target)) openList.close(false);
  });

  function el(tag, cls) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    return e;
  }

  /* The values in a box that takes several, separated by commas. */
  function listed(input) {
    return input.value.split(',').map(function (s) { return s.trim(); }).filter(function (s) { return s !== ''; });
  }

  /**
   * Wraps `input` in a combo offering `choices` ([{ value, label }], the label optional). An option
   * reads as "label, value", or the value alone. `name` names the list and its toggle, such as
   * "API type suggestions". Returns the combo, which takes the input's place when it has one.
   */
  function build(input, choices, name) {
    var combo = el('div', 'cfg-key-combo');
    var listId = 'cfg-suggest-list-' + (++listCount);
    if (input.parentNode) input.parentNode.replaceChild(combo, input);
    combo.appendChild(input);
    input.setAttribute('role', 'combobox');
    input.setAttribute('aria-autocomplete', 'none');
    input.setAttribute('aria-expanded', 'false');
    input.setAttribute('aria-controls', listId);
    input.autocomplete = 'off';

    var toggle = el('button', 'cfg-key-combo-toggle');
    toggle.type = 'button';
    toggle.tabIndex = -1;
    toggle.setAttribute('aria-label', name);
    toggle.setAttribute('aria-controls', listId);
    toggle.setAttribute('aria-expanded', 'false');
    toggle.innerHTML = '<i class="bi bi-chevron-expand" aria-hidden="true"></i>';

    var menu = el('div', 'cfg-view-combo-menu');
    menu.hidden = true;
    var list = el('div', 'cfg-view-combo-list');
    list.id = listId;
    list.setAttribute('role', 'listbox');
    list.setAttribute('aria-label', name);
    choices.forEach(function (choice) {
      var option = el('div', 'cfg-view-combo-option');
      option.setAttribute('role', 'option');
      option.tabIndex = -1;
      option.setAttribute('aria-selected', 'false');
      option.dataset.value = choice.value;
      option.setAttribute('aria-label', choice.label ? choice.label + ', ' + choice.value : choice.value);
      var check = el('i', 'bi bi-check2');
      check.setAttribute('aria-hidden', 'true');
      option.appendChild(check);
      var text = el('span', choice.label ? 'cfg-key-option' : '');
      if (choice.label) {
        var label = el('span');
        label.textContent = choice.label;
        var value = el('span', 'cfg-key-option-key');
        value.textContent = choice.value;
        text.appendChild(label);
        text.appendChild(document.createTextNode(' '));
        text.appendChild(value);
      } else {
        text.textContent = choice.value;
      }
      option.appendChild(text);
      list.appendChild(option);
    });
    menu.appendChild(list);
    combo.appendChild(toggle);
    combo.appendChild(menu);
    return combo;
  }

  /**
   * Gives a combo (from build() or the page) its behaviour. `opts.multiple`, when given, says
   * whether the box takes several comma-separated values at the moment; a chosen value is then
   * added to them rather than replacing them. Returns { close(focusInput), showStart() }.
   */
  function attach(combo, opts) {
    var multiple = (opts && opts.multiple) || function () { return false; };
    var input = combo.querySelector('[role="combobox"]');
    var toggle = combo.querySelector('.cfg-key-combo-toggle');
    var menu = combo.querySelector('.cfg-view-combo-menu');
    var self = { combo: combo, close: close };

    function options() { return Array.prototype.slice.call(menu.querySelectorAll('[role="option"]')); }

    function setExpanded(open) {
      menu.hidden = !open;
      input.setAttribute('aria-expanded', open ? 'true' : 'false');
      toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
      if (open) {
        if (openList && openList !== self) openList.close(false);
        openList = self;
      } else if (openList === self) {
        openList = null;
      }
    }

    /* Shows the start of a single value, so a long key reads "metadata.aiTokenUsage…" rather than
       its end, which the browser shows when it puts the caret after a value it fills in. A box of
       several values keeps the caret at the end, where the next one is typed. */
    function showStart() {
      if (multiple()) return;
      if (document.activeElement === input) input.setSelectionRange(0, 0);
      input.scrollLeft = 0;
    }

    function close(focusInput) {
      setExpanded(false);
      if (focusInput) {
        input.focus();
        showStart();
      }
    }

    /* Opens the list with the values in the box marked; focuses the first marked option (or the
       first option) when `focusOption`, else keeps the focus in the box. */
    function open(focusOption) {
      if (input.disabled) return;
      var inBox = multiple() ? listed(input) : [input.value.trim()];
      var current = null;
      options().forEach(function (o) {
        var on = inBox.indexOf(o.dataset.value) !== -1;
        o.classList.toggle('selected', on);
        o.setAttribute('aria-selected', on ? 'true' : 'false');
        if (on && !current) current = o;
      });
      setExpanded(true);
      if (focusOption) {
        (current || options()[0]).focus();
      } else {
        input.focus();
        showStart();
      }
    }

    function choose(option) {
      var value = option.dataset.value;
      if (multiple()) {
        var values = listed(input);
        if (values.indexOf(value) === -1) values.push(value);
        input.value = values.join(', ');
      } else {
        input.value = value;
      }
      close(true);
      input.dispatchEvent(new Event('input', { bubbles: true }));
    }

    toggle.addEventListener('click', function () {
      if (menu.hidden) open(false); else close(true);
    });
    input.addEventListener('keydown', function (e) {
      if (e.key === 'ArrowDown') { e.preventDefault(); open(true); }
      else if (e.key === 'Escape' && !menu.hidden) { e.preventDefault(); e.stopPropagation(); close(true); }
    });
    menu.addEventListener('click', function (e) {
      var option = e.target.closest('[role="option"]');
      if (option) choose(option);
    });
    menu.addEventListener('keydown', function (e) {
      var option = e.target.closest('[role="option"]');
      if (!option) return;
      var all = options();
      var i = all.indexOf(option);
      if (e.key === 'ArrowDown') { e.preventDefault(); (all[i + 1] || all[0]).focus(); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); if (i <= 0) close(true); else all[i - 1].focus(); }
      else if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); choose(option); }
      else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); close(true); }
      else if (e.key === 'Tab') close(false);
    });

    return { close: close, showStart: showStart };
  }

  window.cfgSuggestCombo = { build: build, attach: attach };
}());
