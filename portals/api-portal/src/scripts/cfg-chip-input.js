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
 * A free-text chip input: values are typed, shown as removable chips, and read
 * back as an array.
 *
 * Lifted out of settings-apis.js, which grew the first one for an API's tags.
 * The settings bundle now has a second caller (a key manager's scopes) and the
 * markup and CSS were already shared (.cfg-chip-input-wrap and friends in
 * settings-layout.css) -- only the behaviour was not, so a fix to one would have
 * silently not reached the other.
 *
 * Deliberately not a <input type="text"> with a hidden mirror field: nothing here
 * posts a form, every caller reads its value through JS at save time, so the
 * chips are the state and `get()` is how it leaves.
 *
 * Loaded as a plain script before its callers, like the rest of this bundle, so
 * it hangs off window rather than exporting a module.
 */
(function () {
  function esc(str) {
    return String(str === undefined || str === null ? '' : str).replace(/[&<>"']/g, function (ch) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch];
    });
  }

  /**
   * @param {object}   opts
   * @param {string}   opts.wrapId     the clickable .cfg-chip-input-wrap
   * @param {string}   opts.chipsId    container the chips render into
   * @param {string}   opts.inputId    the text input chips are typed in
   * @param {RegExp}   [opts.splitOn]  what separates several values in one string
   *                                   (typed, pasted, or passed to set()).
   *                                   Defaults to a comma.
   * @param {string}   [opts.commitKey] an extra single-character key that commits
   *                                   the current value, e.g. ' ' for scopes.
   * @param {function} [opts.label]    chip -> title text for its remove button
   * @returns {{get: function, set: function, clear: function}|null}
   *          null when the markup is absent, so a caller on a page without this
   *          field is a no-op rather than a thrown error.
   */
  function create(opts) {
    var wrap = document.getElementById(opts.wrapId);
    var chipsEl = document.getElementById(opts.chipsId);
    var input = document.getElementById(opts.inputId);
    if (!input || !chipsEl) return null;

    var splitOn = opts.splitOn || /,/;
    var chips = [];

    function render() {
      chipsEl.innerHTML = '';
      chips.forEach(function (value) {
        var chip = document.createElement('span');
        chip.className = 'cfg-chip';
        chip.innerHTML = esc(value) +
          '<button type="button" class="cfg-chip-remove" data-value="' + esc(value) +
          '" title="Remove ' + esc(value) + '"><i class="bi bi-x"></i></button>';
        chip.querySelector('.cfg-chip-remove').addEventListener('click', function (e) {
          e.stopPropagation();
          remove(e.currentTarget.dataset.value);
          input.focus();
        });
        chipsEl.appendChild(chip);
      });
    }

    /* Case-insensitive dedupe, keeping the casing first entered: the same value in
       two casings is a typo every time, and unlike a free-text field the duplicate
       would be plainly visible here. */
    function add(raw) {
      var value = String(raw === undefined || raw === null ? '' : raw).trim();
      if (!value) return;
      var lower = value.toLowerCase();
      if (chips.some(function (c) { return c.toLowerCase() === lower; })) return;
      chips.push(value);
      render();
    }

    function remove(value) {
      chips = chips.filter(function (c) { return c !== value; });
      render();
    }

    /* Splits so a pasted "a, b, c" -- and anything typed in the old
       separator-delimited habit -- still lands as separate chips. */
    function commit() {
      input.value.split(splitOn).forEach(add);
      input.value = '';
    }

    if (wrap) {
      wrap.addEventListener('click', function () { input.focus(); });
    }

    input.addEventListener('keydown', function (e) {
      if (e.key === 'Enter' || e.key === ',' || (opts.commitKey && e.key === opts.commitKey)) {
        // Enter would otherwise submit the surrounding form with the value still
        // uncommitted; a separator key would land in the input as a character.
        e.preventDefault();
        commit();
      } else if (e.key === 'Backspace' && !input.value && chips.length) {
        e.preventDefault();
        remove(chips[chips.length - 1]);
      }
    });

    // Losing focus commits too -- otherwise a typed-but-not-entered value is
    // silently dropped when the user clicks Save.
    input.addEventListener('blur', commit);

    input.addEventListener('paste', function (e) {
      var clip = e.clipboardData || window.clipboardData;
      var text = clip && clip.getData('text');
      // Nothing to split on: let the browser paste normally.
      if (!text || !splitOn.test(text)) return;
      e.preventDefault();
      /* Splice the pasted text in at the caret (replacing any selection) rather than
         committing it on its own, so a half-typed value joins the paste instead of
         being left orphaned in the input: "foo" with "bar, baz" pasted at the end
         commits as "foobar" and "baz" -- what typing those same characters would do. */
      var start = input.selectionStart != null ? input.selectionStart : input.value.length;
      var end = input.selectionEnd != null ? input.selectionEnd : input.value.length;
      input.value = input.value.slice(0, start) + text + input.value.slice(end);
      commit();
    });

    return {
      get: function () { return chips.slice(); },
      set: function (list) {
        chips = [];
        var items = Array.isArray(list)
          ? list
          : (list ? String(list).split(splitOn) : []);
        items.forEach(add);
        render();
        input.value = '';
      },
      clear: function () {
        chips = [];
        render();
        input.value = '';
      },
    };
  }

  window.cfgChipInput = { create: create };
}());
