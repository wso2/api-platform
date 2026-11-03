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
 

(function () {
  // Local fallback for the shared bindFormValidity (defined in alert.js, loaded first
  // on the settings page): keeps save/delete working even if it were ever unavailable,
  // only skipping the disable-until-valid behaviour instead of throwing at init.
  var bindFormValidity = window.bindFormValidity || function () { return function () {}; };

  var editPlanId = null;

  var LIMIT_TYPES = ['REQUEST_COUNT','EVENT_COUNT','BANDWIDTH','TOTAL_TOKEN_COUNT'];
  var TIME_UNITS  = ['MINUTE','HOUR','DAY','MONTH'];

  function v(id) { var e=document.getElementById(id); return e?e.value.trim():''; }

  /* ── limit row builder ── */
  function makeLimitRow(limit) {
    var row = document.createElement('div');
    row.className = 'cfg-limit-row';
    row.style.cssText = 'display:flex;gap:8px;align-items:center;margin-bottom:8px;';

    var typeOpts = LIMIT_TYPES.map(function(t){
      return '<option value="'+t+'"'+(limit&&limit.limitType===t?' selected':'')+'>'+t+'</option>';
    }).join('');
    var unitOpts = '<option value="">— no window —</option>'+TIME_UNITS.map(function(u){
      return '<option value="'+u+'"'+(limit&&limit.timeUnit===u?' selected':'')+'>'+u+'</option>';
    }).join('');

    row.innerHTML =
      '<select class="cfg-form-input cfg-limit-type" style="flex:2;">'+typeOpts+'</select>'+
      '<input type="number" class="cfg-form-input cfg-form-input--mono cfg-limit-count" style="flex:1.5;" '+
        'placeholder="count" value="'+(limit?limit.limitCount:'')+'" />'+
      '<span style="color:var(--text-muted);font-size:.8rem;white-space:nowrap;">/ per</span>'+
      '<input type="number" class="cfg-form-input cfg-form-input--mono cfg-limit-amount" style="flex:.7;" '+
        'placeholder="1" value="'+(limit&&limit.timeAmount!=null?limit.timeAmount:1)+'" min="1" />'+
      '<select class="cfg-form-input cfg-limit-unit" style="flex:1.2;">'+unitOpts+'</select>'+
      '<button type="button" class="cfg-icon-btn cfg-icon-btn--danger cfg-limit-remove-btn" title="Remove">'+
        '<i class="bi bi-x-lg"></i></button>';

    row.querySelector('.cfg-limit-remove-btn').addEventListener('click', function(){ row.remove(); });
    return row;
  }

  document.getElementById('pol-add-limit-btn').addEventListener('click', function(){
    document.getElementById('pol-limits-list').appendChild(makeLimitRow(null));
  });

  function readLimits() {
    var rows = document.querySelectorAll('#pol-limits-list .cfg-limit-row');
    var limits = [];
    rows.forEach(function(row){
      var rawCount = row.querySelector('.cfg-limit-count').value.trim();
      var count = rawCount === '-1' ? -1 : parseInt(rawCount, 10);
      var rawAmount = row.querySelector('.cfg-limit-amount').value.trim();
      var amount = rawAmount === '' ? 1 : parseInt(rawAmount, 10);
      if (isNaN(count) || (count !== -1 && count <= 0)) {
        throw new Error('Limit count must be -1 (unlimited) or a positive number.');
      }
      if (isNaN(amount) || amount <= 0) {
        throw new Error('Limit time amount must be a positive number.');
      }
      limits.push({
        limitType:  row.querySelector('.cfg-limit-type').value,
        limitCount: count,
        timeUnit:   row.querySelector('.cfg-limit-unit').value || null,
        timeAmount: amount,
      });
    });
    return limits;
  }

  /* ── paid plan: a flat rate, or up to 20 usage prices, each on a meter and its own filters ── */
  var AMOUNT = /^\d{1,12}(\.\d{1,12})?$/;
  var MAX_PRICES = 20;
  var MAX_VALUES = 20;
  var MAX_VALUE_LENGTH = 255;
  var RANGE_LABELS = { VOLUME: 'If total units are', GRADUATED: 'For units numbered' };
  // Any operator works on any of the meter's properties. A filter's values are typed, separated by
  // commas for "is one of" and "is not one of"; a comparison takes one whole number. A filter with no
  // values is left out.
  var OPERATORS = [['EQUALS', 'equals'], ['NOT_EQUALS', 'does not equal'], ['IN', 'is one of'],
    ['NOT_IN', 'is not one of'], ['STARTS_WITH', 'starts with'], ['GREATER_THAN', 'more than'],
    ['GREATER_THAN_OR_EQUAL', 'at least'], ['LESS_THAN', 'less than'], ['LESS_THAN_OR_EQUAL', 'at most']];
  var OPERATOR_LABELS = {};
  OPERATORS.forEach(function (op) { OPERATOR_LABELS[op[0]] = op[1]; });
  var MANY = ['IN', 'NOT_IN'];
  var MATCHING = ['EQUALS', 'IN'];
  var EXCLUDING = ['NOT_EQUALS', 'NOT_IN'];
  var COMPARE = ['GREATER_THAN', 'GREATER_THAN_OR_EQUAL', 'LESS_THAN', 'LESS_THAN_OR_EQUAL'];
  var MAX_COMPARE_VALUE = 1000000000000;

  function meters() {
    try { return JSON.parse(document.getElementById('pol-pricing').dataset.meters || '[]') || []; } catch (e) { return []; }
  }

  function meterById(id) {
    return meters().filter(function (m) { return m.id === id; })[0] || null;
  }

  /* What a meter counts: "Requests" or "Tokens". */
  function eventLabel(meter) {
    return meter ? meter.typeLabel || '' : '';
  }

  /* The properties a price may filter on: those of its chosen meter. */
  function properties(box) {
    var meter = meterById(box.querySelector('.pol-price-meter').value);
    return (meter && meter.filterFields ? meter.filterFields : []).map(function (f) { return f.field; });
  }

  /* Whether two property names are the same, ignoring case. */
  function sameName(a, b) {
    return a.toLowerCase() === b.toLowerCase();
  }

  /* A filter as it reads: "API type is one of", "Prompt tokens more than". */
  function filterLabel(filter) {
    return filter.field + ' ' + (OPERATOR_LABELS[filter.operator] || filter.operator);
  }

  function el(tag, cls) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    return e;
  }

  function opt(value, label, selected) {
    var o = document.createElement('option');
    o.value = value; o.textContent = label; o.selected = !!selected;
    return o;
  }

  function moneyInput(cls, value) {
    var wrap = el('span', 'cfg-money');
    var input = el('input', 'cfg-form-input cfg-form-input--mono ' + cls);
    input.type = 'text'; input.inputMode = 'decimal'; input.placeholder = '0.00';
    input.value = value != null ? value : '';
    wrap.appendChild(input);
    return wrap;
  }

  /* A tier: "from – [to]" → per unit + flat fee. Only the last tier is open-ended. */
  function makeTierRow(tier) {
    var row = el('div', 'cfg-tier-row');
    var range = el('div', 'cfg-tier-range');
    var from = el('span', 'cfg-tier-from');
    var upTo = el('input', 'cfg-form-input cfg-form-input--mono cfg-tier-upto');
    upTo.type = 'number'; upTo.min = '1'; upTo.step = '1';
    upTo.value = tier && tier.upTo != null ? tier.upTo : '';
    upTo.addEventListener('input', function () { renumberTiers(row.parentNode); });
    range.appendChild(from); range.appendChild(upTo);
    var arrow = el('span', 'cfg-tier-sign'); arrow.innerHTML = '<i class="bi bi-arrow-right"></i>';
    var plus = el('span', 'cfg-tier-sign'); plus.textContent = '+';
    var remove = el('button', 'cfg-icon-btn cfg-icon-btn--danger');
    remove.type = 'button'; remove.title = 'Remove tier'; remove.innerHTML = '<i class="bi bi-x-lg"></i>';
    remove.addEventListener('click', function () { var list = row.parentNode; row.remove(); renumberTiers(list); });
    row.appendChild(range); row.appendChild(arrow);
    row.appendChild(moneyInput('cfg-tier-unit', tier ? tier.unitAmount : null));
    row.appendChild(plus);
    row.appendChild(moneyInput('cfg-tier-flat', tier ? tier.flatAmount : null));
    row.appendChild(remove);
    return row;
  }

  /* Each tier starts after the one before it; the last has no upper bound; one tier always stays. */
  function renumberTiers(list) {
    var rows = list.querySelectorAll('.cfg-tier-row');
    var start = 1;
    rows.forEach(function (row, i) {
      var last = i === rows.length - 1;
      var upTo = row.querySelector('.cfg-tier-upto');
      row.querySelector('.cfg-tier-from').textContent = start + ' –';
      upTo.disabled = last;
      upTo.placeholder = last ? '∞' : String(start);
      if (last) upTo.value = '';
      row.querySelector('.cfg-icon-btn').style.visibility = rows.length > 1 ? '' : 'hidden';
      var n = parseInt(upTo.value, 10);
      start = Number.isFinite(n) && n >= start ? n + 1 : start + 1;
    });
  }

  function fillMeters(select, selected) {
    select.innerHTML = '';
    // No meter is chosen for the administrator; a price cannot be saved without one.
    select.appendChild(opt('', 'Select a meter', !selected));
    meters().forEach(function (m) { select.appendChild(opt(m.id, m.displayName, m.id === selected)); });
    if (selected && !meterById(selected)) select.appendChild(opt(selected, selected, true));
  }

  /* The values a filter row holds: for "is one of" and "is not one of" the typed ones, separated by
     commas, each once; for any other operator the one typed value. */
  function rowValues(row) {
    var operator = row.querySelector('.pol-filter-operator').value;
    var text = row.querySelector('.pol-filter-values').value;
    if (MANY.indexOf(operator) === -1) return text.trim() ? [text.trim()] : [];
    var seen = [];
    text.split(',').forEach(function (s) {
      var value = s.trim();
      if (value && seen.indexOf(value) === -1) seen.push(value);
    });
    return seen;
  }

  function filterRows(box) {
    return Array.prototype.slice.call(box.querySelectorAll('.pol-filters-list .cfg-filter-row'));
  }

  /* What each of a price's rows holds, empty or not: [{field, operator, values}]. */
  function rowFilters(box) {
    return filterRows(box).map(function (row) {
      return { field: row.dataset.field, operator: row.querySelector('.pol-filter-operator').value, values: rowValues(row) };
    });
  }

  /* The filters of a price: its rows with values. A row left empty counts every value of its property. */
  function givenFilters(box) {
    return rowFilters(box).filter(function (filter) { return filter.values.length > 0; });
  }

  /* A filter row: the property's name, an operator and the value or values it takes. A row whose
     operator compares a number can add a second comparison on the property, for a range; the second
     can be removed. */
  function makeFilterRow(field, filter, second) {
    var row = el('div', 'cfg-filter-row');
    row.dataset.field = field;
    if (second) row.dataset.second = 'true';
    var name = el('span', 'cfg-filter-name');
    name.textContent = field;
    var operator = el('select', 'cfg-form-input pol-filter-operator');
    operator.setAttribute('aria-label', field + ' operator');
    OPERATORS.forEach(function (op) {
      if (second && COMPARE.indexOf(op[0]) === -1) return;
      operator.appendChild(opt(op[0], op[1], filter && op[0] === filter.operator));
    });
    var values = el('input', 'cfg-form-input cfg-form-input--mono pol-filter-values');
    values.type = 'text';
    values.value = filter && filter.values ? filter.values.join(', ') : '';
    var button = el('button', 'cfg-icon-btn' + (second ? ' cfg-icon-btn--danger' : ''));
    button.type = 'button';
    var buttonName = (second ? 'Remove the second comparison on ' : 'Add a second comparison on ') + field;
    button.title = buttonName;
    button.setAttribute('aria-label', buttonName);
    button.innerHTML = second ? '<i class="bi bi-x-lg" aria-hidden="true"></i>' : '<i class="bi bi-plus-lg" aria-hidden="true"></i>';

    function syncValues() {
      var compare = COMPARE.indexOf(operator.value) !== -1;
      var many = MANY.indexOf(operator.value) !== -1;
      values.placeholder = compare ? 'e.g. 333' : many ? 'e.g. GET, POST' : 'Value';
      values.inputMode = compare ? 'numeric' : 'text';
      values.setAttribute('aria-label', field + (many ? ' values' : ' value'));
    }
    syncValues();
    operator.addEventListener('change', function () {
      syncValues();
      if (second) return;
      // A second comparison goes when the first row stops comparing a number.
      var next = row.nextSibling;
      if (COMPARE.indexOf(operator.value) === -1 && next && next.dataset.second && next.dataset.field === field) next.remove();
      updateSecondComparisonButtons(row.parentNode);
    });
    button.addEventListener('click', function () {
      var list = row.parentNode;
      if (second) {
        row.remove();
      } else {
        var next = makeFilterRow(field, null, true);
        list.insertBefore(next, row.nextSibling);
        next.querySelector('.pol-filter-operator').focus();
      }
      updateSecondComparisonButtons(list);
    });
    row.appendChild(name); row.appendChild(operator); row.appendChild(values); row.appendChild(button);
    return row;
  }

  /* A property's first row offers a second comparison while it compares a number and has none yet. */
  function updateSecondComparisonButtons(list) {
    var rows = Array.prototype.slice.call(list.querySelectorAll('.cfg-filter-row'));
    rows.forEach(function (row) {
      if (row.dataset.second) return;
      var hasSecond = rows.some(function (r) { return r.dataset.second && r.dataset.field === row.dataset.field; });
      var compares = COMPARE.indexOf(row.querySelector('.pol-filter-operator').value) !== -1;
      row.querySelector('.cfg-icon-btn').style.visibility = compares && !hasSecond ? '' : 'hidden';
    });
  }

  /* Shows what the chosen meter counts and one filter row per property it lets prices filter on, in
     its order, with a second row where a kept range has two comparisons, filled from `kept`
     ([{field, operator, values}]). A kept filter on a property the meter does not have (a saved one)
     still shows, so saving can say what is wrong. */
  function fillFilters(box, kept) {
    var meter = meterById(box.querySelector('.pol-price-meter').value);
    var names = properties(box);
    var list = box.querySelector('.pol-filters-list');
    list.innerHTML = '';
    function addRows(field) {
      var on = kept.filter(function (f) { return sameName(f.field, field); });
      list.appendChild(makeFilterRow(field, on[0] || null, false));
      if (on[1]) list.appendChild(makeFilterRow(field, on[1], true));
    }
    names.forEach(addRows);
    var others = [];
    kept.forEach(function (f) {
      var known = names.concat(others).some(function (n) { return sameName(n, f.field); });
      if (!known) others.push(f.field);
    });
    others.forEach(addRows);
    updateSecondComparisonButtons(list);
    var none = list.children.length === 0;
    box.querySelector('.pol-price-event').value = eventLabel(meter);
    box.querySelector('.pol-filters-group').style.display = box.querySelector('.pol-price-meter').value ? '' : 'none';
    box.querySelector('.pol-filters-hint').style.display = none ? 'none' : '';
    box.querySelector('.pol-no-filters').style.display = none ? '' : 'none';
  }

  /* Shows the fields of the chosen model. */
  function syncPriceModel(box) {
    var model = box.querySelector('.pol-price-model').value;
    box.querySelectorAll('.pol-model-fields').forEach(function (fields) {
      fields.style.display = fields.dataset.models.split(' ').indexOf(model) >= 0 ? '' : 'none';
    });
    box.querySelector('.pol-tier-range-label').textContent = RANGE_LABELS[model] || '';
    var list = box.querySelector('.pol-tiers-list');
    if ((model === 'VOLUME' || model === 'GRADUATED') && list.children.length === 0) {
      list.appendChild(makeTierRow({ upTo: 1 }));
      list.appendChild(makeTierRow(null));
      renumberTiers(list);
    }
  }

  function priceBoxes() {
    return Array.prototype.slice.call(document.querySelectorAll('#pol-prices-list .cfg-price-box'));
  }

  /* Numbers the prices, as messages name them, and stops adding at the most a plan takes. A usage
     plan needs a price, so the only one has no Remove. */
  function renumberPrices() {
    var boxes = priceBoxes();
    boxes.forEach(function (box, i) {
      box.querySelector('.pol-price-title').textContent = 'Price ' + (i + 1);
      box.querySelector('.pol-price-remove').hidden = boxes.length === 1;
    });
    document.getElementById('pol-add-price-btn').disabled = boxes.length >= MAX_PRICES;
  }

  /* A usage price: a meter, its filters and a pricing model. */
  var priceCount = 0;
  function makePriceBox(price) {
    var box = document.getElementById('pol-price-template').content.firstElementChild.cloneNode(true);
    box.dataset.prefix = 'pol-price-' + (++priceCount) + '-';
    box.querySelectorAll('[data-field]').forEach(function (field) { field.id = box.dataset.prefix + field.dataset.field; });
    box.querySelectorAll('label[data-for]').forEach(function (label) { label.htmlFor = box.dataset.prefix + label.dataset.for; });

    var meter = box.querySelector('.pol-price-meter');
    var model = box.querySelector('.pol-price-model');
    var tiers = box.querySelector('.pol-tiers-list');
    fillMeters(meter, price ? price.meterId : null);
    fillFilters(box, price && price.filters ? price.filters : []);
    model.value = price ? price.pricingModel : 'PER_UNIT';
    box.querySelector('.pol-price-unit-amount').value = price && price.unitAmount != null ? price.unitAmount : '';
    box.querySelector('.pol-price-unit-quantity').value = price && price.unitQuantity ? price.unitQuantity : 1;
    (price && price.tiers ? price.tiers : []).forEach(function (t) { tiers.appendChild(makeTierRow(t)); });
    renumberTiers(tiers);
    syncPriceModel(box);

    // A new meter gets its own filter rows; a property it shares with the old meter keeps what was entered.
    meter.addEventListener('change', function () {
      var names = properties(box);
      fillFilters(box, rowFilters(box).filter(function (f) {
        return names.some(function (n) { return sameName(n, f.field); });
      }));
    });
    model.addEventListener('change', function () { syncPriceModel(box); });
    box.querySelector('.pol-add-tier-btn').addEventListener('click', function () {
      var rows = tiers.querySelectorAll('.cfg-tier-row');
      // The new tier goes before the open-ended last one.
      tiers.insertBefore(makeTierRow(null), rows[rows.length - 1] || null);
      renumberTiers(tiers);
    });
    box.querySelector('.pol-price-remove').addEventListener('click', function () {
      box.remove();
      renumberPrices();
    });
    return box;
  }

  /* The currency field opens a list that can be searched by code or name. The chosen code is
     kept in #pol-currency, which fires change when it is picked. */
  var currencyPicker = (function () {
    var combo = document.getElementById('pol-currency-combo');
    var trigger = document.getElementById('pol-currency-trigger');
    var menu = combo.querySelector('.cfg-currency-menu');
    var search = combo.querySelector('.cfg-view-combo-input');
    var list = document.getElementById('pol-currency-list');
    var empty = combo.querySelector('.cfg-view-combo-empty');
    var field = document.getElementById('pol-currency');

    function options() { return Array.prototype.slice.call(list.querySelectorAll('[role="option"]')); }
    function shown() { return options().filter(function (o) { return !o.hidden; }); }

    function filter(text) {
      var q = text.trim().toLowerCase();
      var any = false;
      options().forEach(function (o) {
        var hit = !q || o.dataset.value.toLowerCase().indexOf(q) !== -1 || o.dataset.name.toLowerCase().indexOf(q) !== -1;
        o.hidden = !hit;
        if (hit) any = true;
      });
      empty.hidden = any;
    }

    function close(focusTrigger) {
      menu.hidden = true;
      trigger.setAttribute('aria-expanded', 'false');
      if (focusTrigger) trigger.focus();
    }

    function open() {
      menu.hidden = false;
      trigger.setAttribute('aria-expanded', 'true');
      search.value = '';
      filter('');
      var selected = list.querySelector('[aria-selected="true"]');
      if (selected) list.scrollTop = selected.offsetTop - list.clientHeight / 2;
      search.focus();
    }

    /* Selects a code. A saved code that is not in the list is added to it, so it still shows. */
    function set(code) {
      var match = options().filter(function (o) { return o.dataset.value === code; })[0];
      if (!match) {
        match = el('div', 'cfg-view-combo-option');
        match.setAttribute('role', 'option');
        match.tabIndex = -1;
        match.dataset.value = code;
        match.dataset.name = '';
        var check = el('i', 'bi bi-check2');
        check.setAttribute('aria-hidden', 'true');
        var label = el('span');
        label.textContent = code;
        match.appendChild(check);
        match.appendChild(label);
        list.appendChild(match);
      }
      options().forEach(function (o) {
        var on = o === match;
        o.classList.toggle('selected', on);
        o.setAttribute('aria-selected', on ? 'true' : 'false');
      });
      trigger.querySelector('.cfg-currency-current').textContent = match.querySelector('span').textContent;
      field.value = code;
    }

    function choose(option) {
      set(option.dataset.value);
      close(true);
      field.dispatchEvent(new Event('change'));
    }

    trigger.addEventListener('click', function () {
      if (menu.hidden) open(); else close(false);
    });
    search.addEventListener('input', function () { filter(search.value); });
    search.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); close(true); }
      else if (e.key === 'Enter') { e.preventDefault(); if (shown()[0]) choose(shown()[0]); }
      else if (e.key === 'ArrowDown') { e.preventDefault(); if (shown()[0]) shown()[0].focus(); }
      else if (e.key === 'Tab') close(false);
    });
    list.addEventListener('click', function (e) {
      var option = e.target.closest('[role="option"]');
      if (option) choose(option);
    });
    list.addEventListener('keydown', function (e) {
      var option = e.target.closest('[role="option"]');
      if (!option) return;
      var visible = shown();
      var i = visible.indexOf(option);
      if (e.key === 'ArrowDown') { e.preventDefault(); (visible[i + 1] || visible[0]).focus(); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); if (i <= 0) search.focus(); else visible[i - 1].focus(); }
      else if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); choose(option); }
      else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); close(true); }
      else if (e.key === 'Tab') close(false);
    });
    document.addEventListener('click', function (e) {
      if (!menu.hidden && !combo.contains(e.target)) close(false);
    });

    return { set: set, close: close };
  }());

  document.getElementById('pol-add-price-btn').addEventListener('click', function () {
    var box = makePriceBox(null);
    document.getElementById('pol-prices-list').appendChild(box);
    renumberPrices();
    box.querySelector('.pol-price-meter').focus();
  });

  /* A paid plan needs billing: without it the switch stays off and says so. */
  document.getElementById('pol-paid').addEventListener('change', function () {
    var notice = document.getElementById('pol-paid-notice');
    if (this.checked && document.getElementById('pol-pricing-fields').dataset.billingReady !== 'true') {
      this.checked = false;
      notice.hidden = false;
      return;
    }
    notice.hidden = true;
    document.getElementById('pol-pricing').style.display = this.checked ? '' : 'none';
  });
  document.getElementById('pol-paid-setup').addEventListener('click', function (e) {
    e.preventDefault();
    closePlanModal();
    if (window.cfgActivatePanel) window.cfgActivatePanel('cfg-billing');
  });
  document.getElementById('pol-meter-setup').addEventListener('click', function (e) {
    e.preventDefault();
    closePlanModal();
    if (window.cfgActivatePanel) window.cfgActivatePanel('cfg-meters');
  });

  function pricingType() {
    return document.querySelector('input[name="pol-pricing-type"]:checked').value;
  }

  /* A flat rate plan shows its amount; a usage based plan its prices, starting with one. Prices need a
     meter: without one, the usage based part says so instead of offering Add price. */
  function syncPricingType() {
    document.querySelectorAll('#pol-pricing .pol-type-fields').forEach(function (fields) {
      fields.style.display = fields.dataset.type === pricingType() ? '' : 'none';
    });
    var noMeters = meters().length === 0;
    document.getElementById('pol-meter-notice').hidden = !noMeters;
    document.getElementById('pol-add-price-btn').style.display = noMeters ? 'none' : '';
    if (pricingType() === 'USAGE_BASED' && priceBoxes().length === 0 && meters().length > 0) {
      document.getElementById('pol-prices-list').appendChild(makePriceBox(null));
      renumberPrices();
    }
  }
  document.querySelectorAll('input[name="pol-pricing-type"]').forEach(function (radio) {
    radio.addEventListener('change', syncPricingType);
  });

  function checkAmount(value, what) {
    if (!AMOUNT.test(value)) throw new Error(what + ' must be an amount such as 0.01.');
    return value;
  }

  /* The values of one filter of a price, checked; a comparison's number is written without leading
     zeros ("0100" is 100). */
  function checkValues(filter, n) {
    var values = filter.values;
    var which = ' for "' + filterLabel(filter) + '".';
    if (values.length === 0) throw new Error('Price ' + n + ' needs a value' + which);
    if (MANY.indexOf(filter.operator) === -1 && values.length > 1) throw new Error('Price ' + n + ' takes one value' + which);
    if (values.length > MAX_VALUES) throw new Error('Price ' + n + ' takes at most ' + MAX_VALUES + ' values' + which);
    if (COMPARE.indexOf(filter.operator) !== -1) {
      if (!/^\d+$/.test(values[0]) || Number(values[0]) > MAX_COMPARE_VALUE) {
        throw new Error('Price ' + n + ' compares with a whole number from 0 to ' + MAX_COMPARE_VALUE + which);
      }
      filter.values = [String(Number(values[0]))];
      return;
    }
    values.forEach(function (value) {
      if (value.length > MAX_VALUE_LENGTH) {
        throw new Error('Price ' + n + ' takes values of at most ' + MAX_VALUE_LENGTH + ' characters' + which);
      }
    });
  }

  /* The whole numbers a price's comparisons on one property allow: { low, high }, empty when low > high. */
  function compareRange(filters, field) {
    var low = 0;
    var high = Infinity;
    filters.forEach(function (f) {
      if (COMPARE.indexOf(f.operator) === -1 || !sameName(f.field, field)) return;
      var v = Number(f.values[0]);
      if (f.operator === 'GREATER_THAN') low = Math.max(low, v + 1);
      else if (f.operator === 'GREATER_THAN_OR_EQUAL') low = Math.max(low, v);
      else if (f.operator === 'LESS_THAN') high = Math.min(high, v - 1);
      else if (f.operator === 'LESS_THAN_OR_EQUAL') high = Math.min(high, v);
    });
    return { low: low, high: high };
  }

  /* The properties some of `filters` name, each once. */
  function namesOf(filters) {
    var names = [];
    filters.forEach(function (f) {
      if (!names.some(function (n) { return sameName(n, f.field); })) names.push(f.field);
    });
    return names;
  }

  /* A usage price as the API takes it. `n` is its number, which messages name it by. */
  function readPrice(box, n) {
    var meterId = box.querySelector('.pol-price-meter').value;
    if (!meterId) throw new Error('Select a meter for price ' + n + '.');
    var names = properties(box);
    var price = {
      meterId: meterId,
      filters: givenFilters(box).map(function (filter) {
        if (!names.some(function (name) { return sameName(name, filter.field); })) {
          throw new Error('Price ' + n + ' filters on ' + filter.field + ', which the meter ' + meterId + ' does not have.');
        }
        checkValues(filter, n);
        return filter;
      }),
      pricingModel: box.querySelector('.pol-price-model').value,
    };
    namesOf(price.filters).forEach(function (field) {
      var on = price.filters.filter(function (f) { return sameName(f.field, field); });
      if (on.some(function (f) { return COMPARE.indexOf(f.operator) === -1; })) {
        if (on.length > 1) throw new Error('Price ' + n + ' filters on ' + field + ' more than once.');
        return;
      }
      var range = compareRange(price.filters, field);
      if (range.low > range.high) throw new Error('Price ' + n + ': the comparisons on ' + field + ' can\'t all be true.');
    });
    if (price.pricingModel === 'PER_UNIT') {
      price.unitAmount = checkAmount(box.querySelector('.pol-price-unit-amount').value.trim(), 'Price ' + n);
      var quantity = parseInt(box.querySelector('.pol-price-unit-quantity').value.trim() || '1', 10);
      if (!Number.isFinite(quantity) || quantity < 1) throw new Error('Units of price ' + n + ' must be a whole number of at least 1.');
      price.unitQuantity = quantity;
      return price;
    }
    price.tiers = [];
    var previous = 0;
    var rows = box.querySelectorAll('.pol-tiers-list .cfg-tier-row');
    rows.forEach(function (row, i) {
      var last = i === rows.length - 1;
      var upTo = row.querySelector('.cfg-tier-upto').value.trim();
      var unit = row.querySelector('.cfg-tier-unit').value.trim();
      var flat = row.querySelector('.cfg-tier-flat').value.trim();
      if (!last && upTo === '') throw new Error('Each tier of price ' + n + ' except the last needs an upper bound.');
      if (!last && !(parseInt(upTo, 10) > previous)) {
        throw new Error('Each tier of price ' + n + ' needs an upper bound greater than the tier before it.');
      }
      if (unit === '' && flat === '') throw new Error('Each tier of price ' + n + ' needs a per-unit price, a flat fee or both.');
      if (!last) previous = parseInt(upTo, 10);
      price.tiers.push({
        upTo: last ? null : parseInt(upTo, 10),
        unitAmount: unit === '' ? null : checkAmount(unit, 'Per-unit price of price ' + n),
        flatAmount: flat === '' ? null : checkAmount(flat, 'Flat fee of price ' + n),
      });
    });
    return price;
  }

  /* Whether the text filters two prices put on one property can match the same value. A "does not
     equal" or "is not one of" filter leaves values the other can match. */
  function textFiltersOverlap(a, b) {
    if (MATCHING.indexOf(b.operator) !== -1 && MATCHING.indexOf(a.operator) === -1) return textFiltersOverlap(b, a);
    if (MATCHING.indexOf(a.operator) !== -1) {
      if (MATCHING.indexOf(b.operator) !== -1) return a.values.some(function (value) { return b.values.indexOf(value) !== -1; });
      if (EXCLUDING.indexOf(b.operator) !== -1) return a.values.some(function (value) { return b.values.indexOf(value) === -1; });
      return a.values.some(function (value) { return value.indexOf(b.values[0]) === 0; });
    }
    if (a.operator === 'STARTS_WITH' && b.operator === 'STARTS_WITH') {
      return a.values[0].indexOf(b.values[0]) === 0 || b.values[0].indexOf(a.values[0]) === 0;
    }
    return true;
  }

  /* Whether one request can be counted by both prices: on every property both filter on, their filters
     can match one value. A property only one of them filters on matches anything for the other. */
  function pricesOverlap(a, b) {
    return namesOf(a.filters).every(function (field) {
      var onA = a.filters.filter(function (f) { return sameName(f.field, field); });
      var onB = b.filters.filter(function (f) { return sameName(f.field, field); });
      if (onB.length === 0) return true;
      var comparesA = onA.every(function (f) { return COMPARE.indexOf(f.operator) !== -1; });
      var comparesB = onB.every(function (f) { return COMPARE.indexOf(f.operator) !== -1; });
      if (!comparesA && !comparesB) return textFiltersOverlap(onA[0], onB[0]);
      if (comparesA && comparesB) {
        var ra = compareRange(a.filters, field);
        var rb = compareRange(b.filters, field);
        return Math.max(ra.low, rb.low) <= Math.min(ra.high, rb.high);
      }
      var range = compareRange((comparesA ? a : b).filters, field);
      var text = (comparesA ? onB : onA)[0];
      if (MATCHING.indexOf(text.operator) === -1) return true;
      return text.values.some(function (value) {
        return /^\d+$/.test(value) && Number(value) >= range.low && Number(value) <= range.high;
      });
    });
  }

  /* Two prices on one meter may not count the same request. */
  function checkOverlap(prices) {
    prices.forEach(function (a, i) {
      prices.forEach(function (b, j) {
        if (j <= i || a.meterId !== b.meterId || !pricesOverlap(a, b)) return;
        throw new Error('Prices ' + (i + 1) + ' and ' + (j + 1) + ' both count the same requests of the meter '
          + a.meterId + '; give them different filters.');
      });
    });
  }

  /* The pricing to save, or null for a free plan. Throws with a message fit for the user. */
  function readPricing() {
    if (!document.getElementById('pol-paid').checked) return null;
    var pricing = {
      pricingType: pricingType(),
      currency: document.getElementById('pol-currency').value,
      billingPeriod: document.getElementById('pol-period').value,
    };
    if (pricing.pricingType === 'FLAT_RATE') {
      var flat = v('pol-flat-amount');
      if (!AMOUNT.test(flat)) throw new Error('Amount must be a number such as 20.00.');
      if (!/[1-9]/.test(flat)) throw new Error('Amount must be more than 0.');
      pricing.flatAmount = flat;
      return pricing;
    }
    pricing.prices = priceBoxes().map(function (box, i) { return readPrice(box, i + 1); });
    if (pricing.prices.length === 0) throw new Error('Add at least one price.');
    if (pricing.prices.length > MAX_PRICES) throw new Error('A plan has at most ' + MAX_PRICES + ' prices.');
    checkOverlap(pricing.prices);
    return pricing;
  }

  function fillPricing(pricing) {
    document.getElementById('pol-paid-notice').hidden = true;
    document.getElementById('pol-paid').checked = !!pricing;
    document.getElementById('pol-pricing').style.display = pricing ? '' : 'none';
    currencyPicker.close(false);
    currencyPicker.set(pricing ? pricing.currency : 'USD');
    document.getElementById('pol-period').value = pricing ? pricing.billingPeriod : 'MONTH';
    var type = pricing ? pricing.pricingType : 'FLAT_RATE';
    document.querySelectorAll('input[name="pol-pricing-type"]').forEach(function (radio) { radio.checked = radio.value === type; });
    document.getElementById('pol-flat-amount').value = pricing && pricing.flatAmount ? pricing.flatAmount : '';
    var list = document.getElementById('pol-prices-list');
    list.innerHTML = '';
    (pricing && pricing.prices ? pricing.prices : []).forEach(function (p) { list.appendChild(makePriceBox(p)); });
    renumberPrices();
    syncPricingType();
  }

  /* ── open / close modal ── */
  function openPlanModal(mode, data) {
    editPlanId = mode === 'edit' ? data.planId : null;
    document.getElementById('cfg-plan-modal-title').textContent = mode === 'edit' ? 'Edit subscription plan' : 'Add subscription plan';
    document.getElementById('cfg-plan-modal-save').textContent  = mode === 'edit' ? 'Save changes' : 'Add plan';
    document.getElementById('pol-display').value = mode === 'edit' ? (data.displayName||'') : '';
    document.getElementById('pol-desc').value    = mode === 'edit' ? (data.description||'') : '';
    document.getElementById('pol-refid').value   = mode === 'edit' ? (data.refId||'')       : '';

    var list = document.getElementById('pol-limits-list');
    list.innerHTML = '';
    var existing = [];
    if (mode === 'edit' && data.limits) {
      try { existing = typeof data.limits === 'string' ? JSON.parse(data.limits) : data.limits; } catch(e){}
    }
    existing.forEach(function(l){ list.appendChild(makeLimitRow(l)); });

    var pricing = null;
    if (mode === 'edit' && data.pricing) {
      try { pricing = typeof data.pricing === 'string' ? JSON.parse(data.pricing) : data.pricing; } catch(e){}
    }
    fillPricing(pricing);
    // Pricing is given only when a plan is created: a saved plan shows its pricing read-only.
    document.getElementById('pol-pricing-fields').disabled = mode === 'edit';

    syncPlanSave();
    document.getElementById('cfg-plan-modal').style.display = 'flex';
    document.getElementById('pol-display').focus();
  }
  function closePlanModal() { document.getElementById('cfg-plan-modal').style.display='none'; editPlanId=null; }

  /* Disable save until the plan Name is filled. */
  var syncPlanSave = bindFormValidity(document.getElementById('cfg-plan-modal-save'), ['pol-display'], function() {
    return document.getElementById('pol-display').value.trim() !== '';
  });

  /* ── save ── */
  document.getElementById('cfg-plan-modal-save').addEventListener('click', async function() {
    var displayName = v('pol-display');
    if (!displayName) { await showAlert('Name is required.', 'error'); return; }

    var limits, pricing;
    try {
      limits = readLimits();
      // Editing never sends pricing: it can't change once a plan is created.
      pricing = editPlanId ? null : readPricing();
    } catch (e) {
      await showAlert(e.message, 'error');
      return;
    }

    var body = {
      displayName: displayName,
      description: v('pol-desc') || undefined,
      limits:      limits,
      refId:       v('pol-refid') || undefined,
    };
    // No `id` on create — the server generates a UUID handle. On edit, send the
    // existing handle so the update targets the right plan (it is not renamed).
    if (editPlanId) body.id = editPlanId;
    // A paid plan is created in one request with its pricing: both are saved, or neither is.
    if (pricing) body.pricing = pricing;

    /* The text to show when the portal refuses the plan. A pricing problem is described in the first
       entry of `errors`, with only "Bad Request" as the message, so that entry is shown, unless the
       message already includes it. */
    async function reasonOf(res) {
      var err = await res.json().catch(function(){ return {}; });
      var message = err.message || err.description || res.statusText;
      var detail = err.errors && err.errors[0] && err.errors[0].message;
      return detail && message.indexOf(detail) === -1 ? detail : message;
    }

    await withButtonBusy(this, editPlanId ? 'Saving…' : 'Adding…', async function() {
      try {
        var res = await fetch(window.apiPortalApi.root('/subscription-plans'), {
          method: editPlanId ? 'PUT' : 'POST',
          headers: { 'Content-Type':'application/json', 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
          body: JSON.stringify(body),
        });
        if (!res.ok) { await showAlert('Plan could not be saved: '+await reasonOf(res), 'error'); return; }
        await showAlert(editPlanId ? 'Plan updated.' : 'Plan created.', 'success');
        window.location.reload();
      } catch(e) { await showAlert('The portal could not be reached. Check your connection and try again.', 'error'); }
    });
  });

  document.getElementById('cfg-plan-modal-close').addEventListener('click', closePlanModal);
  document.getElementById('cfg-plan-modal-cancel').addEventListener('click', closePlanModal);
  document.getElementById('cfg-plan-modal').addEventListener('click', function(e){ if(e.target===this) closePlanModal(); });
  document.getElementById('cfg-add-plan-btn').addEventListener('click', function() { openPlanModal('add'); });

  /* ── edit / delete delegation ── */
  var pendingDelPlanId = null;
  document.addEventListener('click', function(e) {
    if (e.target.closest('.cfg-plan-edit-btn')) {
      var btn = e.target.closest('.cfg-plan-edit-btn');
      openPlanModal('edit', {
        planId:      btn.dataset.id,
        planName:    btn.dataset.name,
        displayName: btn.dataset.display,
        description: btn.dataset.desc,
        limits:      btn.dataset.limits,
        pricing:     btn.dataset.pricing,
        refId:       btn.dataset.refid,
      });
      return;
    }
    if (e.target.closest('.cfg-plan-delete-btn')) {
      btn = e.target.closest('.cfg-plan-delete-btn');
      // A plan that an API offers, or that has subscriptions, can't be deleted: the dialog says it is in
      // use, and Delete is disabled.
      var name = btn.dataset.display || btn.dataset.name;
      var apis = Number(btn.dataset.apis) || 0;
      var subscriptions = Number(btn.dataset.subscriptions) || 0;
      var blocked = apis > 0 || subscriptions > 0;
      document.getElementById('cfg-del-plan-ask').style.display = blocked ? 'none' : '';
      document.getElementById('cfg-del-plan-blocked').style.display = blocked ? '' : 'none';
      document.getElementById('cfg-del-plan-confirm').disabled = blocked;
      pendingDelPlanId = blocked ? null : btn.dataset.id;
      if (blocked) {
        var strong = document.createElement('strong');
        strong.textContent = name;
        document.getElementById('cfg-del-plan-blocked')
          .replaceChildren(strong, document.createTextNode(' is in use and can\'t be deleted.'));
      } else {
        document.getElementById('cfg-del-plan-name-txt').textContent = name;
      }
      document.getElementById('cfg-delete-plan-modal').style.display = 'flex';
      return;
    }
  });

  document.getElementById('cfg-del-plan-cancel').addEventListener('click', function() {
    document.getElementById('cfg-delete-plan-modal').style.display = 'none';
  });
  document.getElementById('cfg-delete-plan-modal').addEventListener('click', function(e){ if(e.target===this) this.style.display='none'; });
  document.getElementById('cfg-del-plan-confirm').addEventListener('click', function() {
    if (!pendingDelPlanId) return;
    withButtonBusy(this, 'Deleting…', async function() {
      try {
        var res = await fetch(window.apiPortalApi.root('/subscription-plans/'+encodeURIComponent(pendingDelPlanId)), {
          method: 'DELETE',
          headers: { 'X-CSRF-Token': window.apiPortalApi.csrfToken() },
        });
        if (res.ok || res.status===204) {
          await showAlert('Plan deleted.', 'success');
          window.location.reload();
          return;
        }
        var err = await res.json().catch(function(){ return {}; });
        await showAlert('Plan could not be deleted: '+(err.message||err.description||res.statusText), 'error');
      } catch(e) { await showAlert('The portal could not be reached. Check your connection and try again.', 'error'); }
      document.getElementById('cfg-delete-plan-modal').style.display = 'none';
      pendingDelPlanId = null;
    });
  });
}());
