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

  var TIME_UNITS  = ['MINUTE','HOUR','DAY','MONTH'];
  var BUILT_IN_IDS = { 'request-count': 'requests', 'tool-call-count': 'tool-calls', 'token-count': 'tokens' };
  var CUSTOM_ID = /^[a-z][a-z0-9-]{0,62}$/;
  var MAX_CONDITIONS = 20;
  var MAX_WINDOWS = 10;

  /* The condition types: what the chooser cards and the Add condition menu offer. */
  var TYPES = [
    { type: 'request-count', name: 'Request count', icon: 'bi-activity', scope: 'REST API only',
      desc: 'Requests per period, for the whole REST API.', menu: 'REST APIs only. Counts every request.' },
    { type: 'tool-call-count', name: 'Tool-call count', icon: 'bi-wrench', scope: 'MCP only',
      desc: 'MCP tools/call per period, for the whole MCP proxy. initialize and tools/list are not counted.',
      menu: 'MCP proxies only. Counts tools/call.' },
    { type: 'token-count', name: 'Token count', icon: 'bi-coin', scope: 'LLM only',
      desc: 'LLM tokens per period: total, prompt or completion.', menu: 'LLM proxies only. Counts model tokens.' },
    { type: 'custom', name: 'Custom', icon: 'bi-puzzle',
      desc: 'Your own key-value pairs, read by a policy your team writes.',
      menu: 'An ID and key-value pairs, read by a policy your team writes.' },
  ];
  var TYPE_OF = {};
  TYPES.forEach(function (t) { TYPE_OF[t.type] = t; });

  var UNIT_WORDS = { MINUTE: 'minute', HOUR: 'hour', DAY: 'day', MONTH: 'month' };
  var WHAT = { 'request-count': 'requests', 'tool-call-count': 'tool calls' };
  var TOKEN_WORDS = { total: 'tokens', prompt: 'prompt tokens', completion: 'completion tokens' };

  function v(id) { var e=document.getElementById(id); return e?e.value.trim():''; }

  function per(amount, unit) {
    var word = UNIT_WORDS[unit] || String(unit).toLowerCase();
    return amount > 1 ? amount + ' ' + word + 's' : word;
  }

  function icon(name) {
    var i = document.createElement('i');
    i.className = 'bi ' + name;
    i.setAttribute('aria-hidden', 'true');
    return i;
  }

  var list = document.getElementById('pln-cond-list');
  var chooser = document.getElementById('pln-chooser');
  var addBtn = document.getElementById('pln-add-cond');
  var menu = document.getElementById('pln-cond-menu');

  function condEls() { return Array.prototype.slice.call(list.querySelectorAll('.pln-cond')); }
  function hasType(type) { return condEls().some(function (c) { return c.dataset.type === type; }); }

  /* ── one limit of a built-in condition: count per time window ── */
  function makeWindowRow(w) {
    var row = document.createElement('div');
    row.className = 'cfg-limit-row pln-window';
    var unitOpts = TIME_UNITS.map(function(u){
      return '<option value="'+u+'"'+(w&&w.timeUnit===u?' selected':'')+'>'+u+'</option>';
    }).join('');
    row.innerHTML =
      '<input type="number" class="cfg-form-input cfg-form-input--mono cfg-limit-count" aria-label="Count" '+
        'placeholder="count" min="1" value="'+(w?w.count:'')+'" />'+
      '<span class="pln-window-per">per</span>'+
      '<input type="number" class="cfg-form-input cfg-form-input--mono cfg-limit-amount" aria-label="Window length" '+
        'placeholder="1" value="'+(w&&w.timeAmount!=null?w.timeAmount:1)+'" min="1" />'+
      '<select class="cfg-form-input cfg-limit-unit" aria-label="Window unit">'+unitOpts+'</select>'+
      '<button type="button" class="cfg-icon-btn cfg-icon-btn--danger cfg-limit-remove-btn" title="Remove limit" aria-label="Remove limit">'+
        '<i class="bi bi-x-lg"></i></button>';
    row.querySelector('.cfg-limit-remove-btn').addEventListener('click', function(){ row.remove(); refresh(); });
    return row;
  }

  /* ── one key-value pair of a custom condition ── */
  function makePairRow(key, value) {
    var row = document.createElement('div');
    row.className = 'pln-pair';
    var k = document.createElement('input');
    k.type = 'text'; k.className = 'cfg-form-input cfg-form-input--mono pln-pair-key'; k.placeholder = 'key';
    k.setAttribute('aria-label', 'Key'); k.value = key || '';
    var eq = document.createElement('span'); eq.className = 'pln-window-per'; eq.textContent = '=';
    var val = document.createElement('input');
    val.type = 'text'; val.className = 'cfg-form-input cfg-form-input--mono pln-pair-value'; val.placeholder = 'value';
    val.setAttribute('aria-label', 'Value');
    val.value = value === undefined ? '' : (typeof value === 'string' ? value : JSON.stringify(value));
    // The saved value, so an untouched one is sent back as it was ("00123" stays a string).
    if (value !== undefined) {
      row.dataset.savedText = val.value;
      row.dataset.savedValue = JSON.stringify(value);
    }
    var rm = document.createElement('button');
    rm.type = 'button'; rm.className = 'cfg-icon-btn cfg-icon-btn--danger'; rm.title = 'Remove value';
    rm.setAttribute('aria-label', 'Remove value'); rm.appendChild(icon('bi-x-lg'));
    rm.addEventListener('click', function () { row.remove(); refresh(); });
    row.appendChild(k); row.appendChild(eq); row.appendChild(val); row.appendChild(rm);
    return row;
  }

  /* A typed value: a number, true or false, a JSON object or list, or else the text as it is. */
  function parseValue(text) {
    if (/^-?\d+(\.\d+)?$/.test(text)) return Number(text);
    if (text === 'true' || text === 'false') return text === 'true';
    if (/^[[{]/.test(text)) { try { return JSON.parse(text); } catch (e) { /* plain text */ } }
    return text;
  }

  /* ── the one-line summary a condition row shows ── */
  function windowsOf(el) {
    var out = [];
    el.querySelectorAll('.cfg-limit-row').forEach(function (row) {
      var count = parseInt(row.querySelector('.cfg-limit-count').value, 10);
      if (!(count >= 1)) return;
      var amount = parseInt(row.querySelector('.cfg-limit-amount').value, 10) || 1;
      out.push({ count: count, amount: amount, unit: row.querySelector('.cfg-limit-unit').value });
    });
    return out;
  }

  function whatOf(el) {
    var tokenType = el.querySelector('.pol-condition-token-type');
    return tokenType ? TOKEN_WORDS[tokenType.value] : WHAT[el.dataset.type];
  }

  function summaryOf(el) {
    if (el.dataset.type === 'custom') {
      var pairs = [];
      el.querySelectorAll('.pln-pair').forEach(function (row) {
        var k = row.querySelector('.pln-pair-key').value.trim();
        if (k) pairs.push(k + ' = ' + row.querySelector('.pln-pair-value').value.trim());
      });
      return pairs.length ? pairs.join(' · ') : 'No values yet';
    }
    var what = whatOf(el);
    var parts = windowsOf(el).map(function (w) { return w.count.toLocaleString() + ' ' + what + ' per ' + per(w.amount, w.unit); });
    if (!parts.length) return 'No limit yet';
    if (el.querySelector('.pol-condition-on-exceed').value === 'allow') parts.push('overage allowed');
    return parts.join(' · ');
  }

  /* ── a condition row: header with its summary, and its editor below ── */
  var condCount = 0;
  function makeCondition(type, c, open) {
    var t = TYPE_OF[type];
    var params = (c && c.params) || {};
    var el = document.createElement('div');
    el.className = 'pln-cond';
    el.dataset.type = type;

    var head = document.createElement('div');
    head.className = 'pln-cond-row';
    var toggle = document.createElement('button');
    toggle.type = 'button'; toggle.className = 'pln-cond-toggle';
    toggle.appendChild(icon('bi-chevron-right'));
    var ic = document.createElement('span'); ic.className = 'pln-cond-icon'; ic.appendChild(icon(t.icon));
    var text = document.createElement('span'); text.className = 'pln-cond-text';
    var title = document.createElement('span'); title.className = 'pln-cond-title'; title.textContent = t.name;
    var sum = document.createElement('span'); sum.className = 'pln-cond-summary';
    text.appendChild(title); text.appendChild(sum);
    toggle.appendChild(ic); toggle.appendChild(text);
    var chip = document.createElement('code'); chip.className = 'pln-cond-chip';
    var del = document.createElement('button');
    del.type = 'button'; del.className = 'cfg-icon-btn cfg-icon-btn--danger pln-cond-delete';
    del.title = 'Remove condition'; del.setAttribute('aria-label', 'Remove ' + t.name + ' condition');
    del.appendChild(icon('bi-trash'));
    head.appendChild(toggle); head.appendChild(chip); head.appendChild(del);

    var body = document.createElement('div');
    body.className = 'pln-cond-body';
    var bodyId = 'pln-cond-body-' + (++condCount);
    body.id = bodyId;
    toggle.setAttribute('aria-controls', bodyId);

    if (type === 'custom') {
      var idGroup = document.createElement('div'); idGroup.className = 'cfg-form-group';
      var idLabel = document.createElement('label'); idLabel.textContent = 'ID'; idLabel.htmlFor = bodyId + '-id';
      var idInput = document.createElement('input');
      idInput.type = 'text'; idInput.id = bodyId + '-id'; idInput.className = 'cfg-form-input cfg-form-input--mono pln-custom-id';
      idInput.placeholder = 'e.g. business-hours'; idInput.maxLength = 63; idInput.value = (c && c.id) || '';
      var idHint = document.createElement('p'); idHint.className = 'cfg-field-hint';
      idHint.textContent = 'Lowercase letters, digits and hyphens, starting with a letter. The policy finds the condition by this ID.';
      idGroup.appendChild(idLabel); idGroup.appendChild(idInput); idGroup.appendChild(idHint);
      var pairs = document.createElement('div'); pairs.className = 'pln-pairs';
      var keys = Object.keys(params);
      (keys.length ? keys : ['']).forEach(function (k) { pairs.appendChild(makePairRow(k, k ? params[k] : undefined)); });
      var addPair = document.createElement('button');
      addPair.type = 'button'; addPair.className = 'cfg-btn-secondary cfg-btn-sm';
      addPair.appendChild(icon('bi-plus')); addPair.appendChild(document.createTextNode(' Add value'));
      addPair.addEventListener('click', function () {
        var row = makePairRow('', undefined); pairs.appendChild(row); row.querySelector('input').focus(); refresh();
      });
      body.appendChild(idGroup); body.appendChild(pairs); body.appendChild(addPair);
    } else {
      var controls = document.createElement('div'); controls.className = 'pln-cond-controls';
      if (type === 'token-count') {
        var tt = document.createElement('select');
        tt.className = 'cfg-form-input pol-condition-token-type'; tt.setAttribute('aria-label', 'Tokens counted');
        [['total', 'Total tokens'], ['prompt', 'Prompt tokens'], ['completion', 'Completion tokens']].forEach(function (o) {
          var op = document.createElement('option'); op.value = o[0]; op.textContent = o[1]; tt.appendChild(op);
        });
        tt.value = params.tokenType || 'total';
        controls.appendChild(tt);
      }
      var ex = document.createElement('select');
      ex.className = 'cfg-form-input pol-condition-on-exceed'; ex.setAttribute('aria-label', 'When a limit is reached');
      [['reject', 'Reject when reached'], ['allow', 'Allow when reached']].forEach(function (o) {
        var op = document.createElement('option'); op.value = o[0]; op.textContent = o[1]; ex.appendChild(op);
      });
      ex.value = params.onExceed || 'reject';
      controls.appendChild(ex);
      var windows = document.createElement('div'); windows.className = 'pln-windows';
      var limits = params.limits && params.limits.length ? params.limits : [null];
      limits.forEach(function (w) { windows.appendChild(makeWindowRow(w)); });
      var addLimit = document.createElement('button');
      addLimit.type = 'button'; addLimit.className = 'cfg-btn-secondary cfg-btn-sm';
      addLimit.appendChild(icon('bi-plus')); addLimit.appendChild(document.createTextNode(' Add limit'));
      addLimit.addEventListener('click', function () {
        if (windows.children.length >= MAX_WINDOWS) return;
        var row = makeWindowRow(null); windows.appendChild(row); row.querySelector('.cfg-limit-count').focus(); refresh();
      });
      body.appendChild(controls); body.appendChild(windows); body.appendChild(addLimit);
    }

    el.appendChild(head);
    el.appendChild(body);

    function setOpen(on) {
      el.classList.toggle('pln-cond--open', on);
      toggle.setAttribute('aria-expanded', on ? 'true' : 'false');
      body.hidden = !on;
    }
    toggle.addEventListener('click', function () { setOpen(body.hidden); });
    del.addEventListener('click', function () { el.remove(); syncConditions(); refresh(); });
    setOpen(!!open);
    return el;
  }
  function addCondition(type) {
    if (condEls().length >= MAX_CONDITIONS) { showAlert('A plan can have at most ' + MAX_CONDITIONS + ' conditions.', 'error'); return; }
    if (type !== 'custom' && hasType(type)) return;
    var el = makeCondition(type, null, true);
    list.appendChild(el);
    chooser.hidden = true;
    closeMenu();
    syncConditions();
    refresh();
    var first = el.querySelector('.pln-cond-body input, .pln-cond-body select');
    if (first) first.focus();
  }

  /* The chooser shows while the plan has no conditions (unless dismissed); Add condition once it has some. */
  function syncConditions() {
    var n = condEls().length;
    document.getElementById('pln-cond-count').textContent = n;
    if (n === 0 && !chooserDismissed) chooser.hidden = false;
    if (n > 0) chooser.hidden = true;
    addBtn.hidden = !chooser.hidden;
    document.getElementById('pln-cond-empty').hidden = n > 0 || !chooser.hidden;
  }
  var chooserDismissed = false;

  /* ── the type cards, and the Add condition menu ── */
  TYPES.forEach(function (t) {
    var card = document.createElement('button');
    card.type = 'button'; card.className = 'pln-type-card'; card.dataset.type = t.type;
    var top = document.createElement('span'); top.className = 'pln-type-top';
    var ic = document.createElement('span'); ic.className = 'pln-cond-icon'; ic.appendChild(icon(t.icon));
    top.appendChild(ic);
    if (t.scope) { var sc = document.createElement('span'); sc.className = 'pln-scope'; sc.textContent = t.scope; top.appendChild(sc); }
    var nm = document.createElement('span'); nm.className = 'pln-type-name'; nm.textContent = t.name;
    var ds = document.createElement('span'); ds.className = 'pln-type-desc'; ds.textContent = t.desc;
    card.appendChild(top); card.appendChild(nm); card.appendChild(ds);
    card.addEventListener('click', function () { addCondition(t.type); });
    document.getElementById('pln-chooser-grid').appendChild(card);
  });
  document.getElementById('pln-chooser-cancel').addEventListener('click', function () {
    chooser.hidden = true; chooserDismissed = true; syncConditions();
  });

  function closeMenu() { menu.hidden = true; addBtn.setAttribute('aria-expanded', 'false'); }
  function openMenu() {
    menu.innerHTML = '';
    TYPES.forEach(function (t) {
      var added = t.type !== 'custom' && hasType(t.type);
      var item = document.createElement('button');
      item.type = 'button'; item.className = 'pln-menu-item'; item.setAttribute('role', 'menuitem');
      item.disabled = added;
      var ic = document.createElement('span'); ic.className = 'pln-cond-icon'; ic.appendChild(icon(t.icon));
      var text = document.createElement('span'); text.className = 'pln-menu-text';
      var nm = document.createElement('span'); nm.className = 'pln-type-name'; nm.textContent = t.name;
      if (added) { var b = document.createElement('span'); b.className = 'pln-added'; b.textContent = 'Added'; nm.appendChild(b); }
      var ds = document.createElement('span'); ds.className = 'pln-type-desc'; ds.textContent = t.menu;
      text.appendChild(nm); text.appendChild(ds);
      item.appendChild(ic); item.appendChild(text);
      item.addEventListener('click', function () { addCondition(t.type); });
      menu.appendChild(item);
    });
    menu.hidden = false;
    addBtn.setAttribute('aria-expanded', 'true');
    var first = menu.querySelector('.pln-menu-item:not(:disabled)');
    if (first) first.focus();
  }
  addBtn.addEventListener('click', function () { if (menu.hidden) openMenu(); else closeMenu(); });
  document.addEventListener('click', function (e) {
    if (!menu.hidden && !menu.contains(e.target) && !addBtn.contains(e.target)) closeMenu();
  });
  menu.addEventListener('keydown', function (e) {
    var items = Array.prototype.slice.call(menu.querySelectorAll('.pln-menu-item:not(:disabled)'));
    var i = items.indexOf(document.activeElement);
    if (e.key === 'Escape') { e.preventDefault(); closeMenu(); addBtn.focus(); }
    else if (e.key === 'ArrowDown') { e.preventDefault(); (items[i + 1] || items[0]).focus(); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); (items[i - 1] || items[items.length - 1]).focus(); }
  });

  /* Shows the plan's conditions, closed; a plan saved before conditions has its request and total token limits as conditions. */
  function fillConditions(conditions) {
    list.innerHTML = '';
    (conditions || []).forEach(function (c) {
      var type = TYPE_OF[c.type] ? c.type : 'custom';
      list.appendChild(makeCondition(type, c, false));
    });
    chooser.hidden = (conditions || []).length > 0;
    syncConditions();
  }

  /* The conditions as the API takes them, in their order. Throws with a message fit for the user. */
  function readConditions() {
    var seen = Object.create(null);
    return condEls().map(function (el) {
      var t = TYPE_OF[el.dataset.type];
      if (el.dataset.type === 'custom') {
        var id = el.querySelector('.pln-custom-id').value.trim();
        if (!CUSTOM_ID.test(id)) throw new Error('A custom condition needs an ID of lowercase letters, digits and hyphens, starting with a letter.');
        if (Object.values(BUILT_IN_IDS).indexOf(id) !== -1) throw new Error(id + ' is the ID of a built-in condition; choose another.');
        if (seen[id]) throw new Error('Two custom conditions have the ID ' + id + '.');
        seen[id] = true;
        var params = {};
        el.querySelectorAll('.pln-pair').forEach(function (row) {
          var k = row.querySelector('.pln-pair-key').value.trim();
          var val = row.querySelector('.pln-pair-value').value.trim();
          if (!k && !val) return;
          if (!k) throw new Error('Each value of the condition ' + id + ' needs a key.');
          if (Object.prototype.hasOwnProperty.call(params, k)) throw new Error('The condition ' + id + ' has the key ' + k + ' twice.');
          params[k] = row.dataset.savedText === val ? JSON.parse(row.dataset.savedValue) : parseValue(val);
        });
        return { id: id, type: 'custom', params: params };
      }
      var limits = [];
      el.querySelectorAll('.cfg-limit-row').forEach(function (row) {
        var rawCount = row.querySelector('.cfg-limit-count').value.trim();
        var rawAmount = row.querySelector('.cfg-limit-amount').value.trim();
        var count = /^\d+$/.test(rawCount) ? parseInt(rawCount, 10) : NaN;
        var amount = rawAmount === '' ? 1 : (/^\d+$/.test(rawAmount) ? parseInt(rawAmount, 10) : NaN);
        if (!(count >= 1)) throw new Error(t.name + ': each limit needs a count of at least 1.');
        if (!(amount >= 1)) throw new Error(t.name + ': each limit needs a time window of at least 1.');
        limits.push({ count: count, timeAmount: amount, timeUnit: row.querySelector('.cfg-limit-unit').value });
      });
      if (limits.length === 0) throw new Error(t.name + ' needs at least one limit, or remove it.');
      var p = { limits: limits, onExceed: el.querySelector('.pol-condition-on-exceed').value };
      var tokenType = el.querySelector('.pol-condition-token-type');
      if (tokenType) p.tokenType = tokenType.value;
      return { id: BUILT_IN_IDS[el.dataset.type], type: el.dataset.type, params: p };
    });
  }

  /* Each row's summary, and the custom ID chip. */
  function refreshConditions() {
    condEls().forEach(function (el) {
      el.querySelector('.pln-cond-summary').textContent = summaryOf(el);
      var chip = el.querySelector('.pln-cond-chip');
      var idInput = el.querySelector('.pln-custom-id');
      chip.textContent = idInput ? idInput.value.trim() : '';
      chip.hidden = !chip.textContent;
    });
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

  /* ── list / form view swap (in-panel, as the Webhooks panel does) ── */
  var listView = document.getElementById('pln-list-view');
  var formView = document.getElementById('pln-form-view');
  var plan = null;

  function plansData() {
    try { return JSON.parse(document.getElementById('cfg-plans-data').textContent) || []; } catch (e) { return []; }
  }

  function showPlanForm(saved) {
    plan = saved || null;
    editPlanId = plan ? plan.planId : null;
    document.getElementById('pln-form-title').textContent = plan ? 'Edit subscription plan' : 'Add subscription plan';
    document.getElementById('pol-save').textContent = plan ? 'Save changes' : 'Add plan';
    document.getElementById('pol-display').value = plan ? (plan.displayName || '') : '';
    document.getElementById('pol-desc').value = plan ? (plan.description || '') : '';
    document.getElementById('pol-refid').value = plan ? (plan.refId || '') : '';
    chooserDismissed = false;
    fillConditions(plan ? plan.conditions : []);
    fillPricing(plan ? plan.pricing : null);
    // Pricing is given only when a plan is created: a saved plan shows its pricing read-only.
    document.getElementById('pol-pricing-fields').disabled = !!plan;
    syncPlanSave();
    refresh();
    listView.style.display = 'none';
    formView.style.display = 'block';
    // The panel scrolls, so returning from a long list would otherwise open the form halfway down.
    formView.scrollIntoView({ block: 'start' });
    document.getElementById('pol-display').focus();
  }

  function showPlanList() {
    closeMenu();
    formView.style.display = 'none';
    listView.style.display = 'block';
    plan = null;
    editPlanId = null;
  }

  document.getElementById('cfg-add-plan-btn').addEventListener('click', function () { showPlanForm(null); });
  document.addEventListener('click', function (e) {
    var btn = e.target.closest('.cfg-plan-edit-btn');
    if (!btn) return;
    var saved = plansData().filter(function (p) { return p.planId === btn.dataset.id; })[0];
    if (saved) showPlanForm(saved);
  });
  document.getElementById('pln-form-back').addEventListener('click', function (e) { e.preventDefault(); showPlanList(); });
  document.getElementById('pol-cancel').addEventListener('click', showPlanList);
  // Setting up billing or a meter leaves the form for that panel.
  [['pol-paid-setup', 'cfg-billing'], ['pol-meter-setup', 'cfg-meters']].forEach(function (link) {
    document.getElementById(link[0]).addEventListener('click', function (e) {
      e.preventDefault();
      showPlanList();
      if (window.cfgActivatePanel) window.cfgActivatePanel(link[1]);
    });
  });

  /* Disable save until the plan Name is filled. */
  var syncPlanSave = bindFormValidity(document.getElementById('pol-save'), ['pol-display'], function() {
    return document.getElementById('pol-display').value.trim() !== '';
  });

  /* ── save ── */
  document.getElementById('pol-save').addEventListener('click', async function() {
    var displayName = v('pol-display');
    if (!displayName) { await showAlert('Name is required.', 'error'); return; }

    var conditions, pricing;
    try {
      conditions = readConditions();
      // Editing never sends pricing: it can't change once a plan is created.
      pricing = editPlanId ? null : readPricing();
    } catch (e) {
      await showAlert(e.message, 'error');
      return;
    }

    var body = {
      displayName: displayName,
      description: v('pol-desc') || undefined,
      conditions:  conditions,
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
        // Back to the plans list, now with the saved plan.
        window.location.hash = 'cfg-plans';
        window.location.reload();
      } catch(e) { await showAlert('The portal could not be reached. Check your connection and try again.', 'error'); }
    });
  });

  /* ── preview: the plan as subscribers read it ── */
  function previewLimits() {
    var items = [];
    condEls().forEach(function (el) {
      // Custom conditions are for policies; subscribers don't see them.
      if (el.dataset.type === 'custom') return;
      var what = whatOf(el);
      var allow = el.querySelector('.pol-condition-on-exceed').value === 'allow';
      windowsOf(el).forEach(function (w) {
        items.push({ text: w.count.toLocaleString('en-US') + ' ' + what + ' / ' + per(w.amount, w.unit), allow: allow });
      });
    });
    return items;
  }

  function previewPrice() {
    // A saved plan's pricing can't change: show it as the plans list does.
    if (plan && plan.pricingText) return plan.pricingText;
    if (!document.getElementById('pol-paid').checked) return 'Free';
    var period = document.getElementById('pol-period');
    var every = period.options[period.selectedIndex] ? period.options[period.selectedIndex].text.toLowerCase() : '';
    if (pricingType() === 'FLAT_RATE') {
      var amount = v('pol-flat-amount');
      return document.getElementById('pol-currency').value + ' ' + (amount || '0.00') + (every ? ' · ' + every : '');
    }
    return 'Usage based · ' + document.getElementById('pol-currency').value;
  }

  function refresh() {
    refreshConditions();
    document.getElementById('pln-preview-name').textContent = v('pol-display') || 'Plan name';
    document.getElementById('pln-preview-desc').textContent = v('pol-desc');
    var list = document.getElementById('pln-preview-limits');
    list.innerHTML = '';
    var items = previewLimits();
    if (items.length === 0) {
      var none = document.createElement('li');
      none.className = 'pln-preview-none';
      none.textContent = 'Unlimited';
      list.appendChild(none);
    }
    items.forEach(function (item) {
      var li = document.createElement('li');
      li.textContent = item.text;
      if (item.allow) {
        var tag = document.createElement('span');
        tag.className = 'pln-preview-tag';
        tag.textContent = 'overage allowed';
        li.appendChild(tag);
      }
      list.appendChild(li);
    });
    document.getElementById('pln-preview-price').textContent = previewPrice();
  }

  var form = document.getElementById('pol-form');
  form.addEventListener('input', refresh);
  form.addEventListener('change', refresh);
  // The currency picker's hidden field fires a change that doesn't bubble.
  document.getElementById('pol-currency').addEventListener('change', refresh);
}());
