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

(function () {
  var STORAGE_KEY = 'dp_ob_dismissed';

  var overlay       = document.getElementById('dp-onboarding');
  if (!overlay) return;
  var BASE_URL    = overlay.dataset.baseUrl || '';
  var SEED_URL    = BASE_URL + '/apis/seed-samples';
  var introPanel    = document.getElementById('dp-ob-intro');
  var deployPanel   = document.getElementById('dp-ob-deploying');
  var donePanel     = document.getElementById('dp-ob-done');
  var deployBtn     = document.getElementById('dp-ob-deploy-btn');
  var skipBtn       = document.getElementById('dp-ob-skip-btn');
  var progressFill  = document.getElementById('dp-ob-progress-fill');
  var progressLabel = document.getElementById('dp-ob-progress-label');
  var curCard       = document.getElementById('dp-ob-cur-card');
  var curSpinner    = document.getElementById('dp-ob-cur-spinner');
  var curCheck      = document.getElementById('dp-ob-cur-check');
  var curNameEl     = document.getElementById('dp-ob-cur-name');
  var curMetaEl     = document.getElementById('dp-ob-cur-meta');
  var doneMsgEl     = document.getElementById('dp-ob-done-msg');
  var doneDetailEl  = document.getElementById('dp-ob-done-detail');

  // Sample APIs on the filesystem — drives the progress animation
  var SAMPLES = [
    { name: 'Ping API',               meta: 'REST · v1.0'   },
    { name: 'Booking API',            meta: 'REST · v1.0'   },
    { name: 'Catalog API',            meta: 'REST · v1.0'   },
    { name: 'Swagger Petstore',       meta: 'REST · v1.0.7' },
    { name: 'Countries GraphQL API',  meta: 'GraphQL · v1.0'},
    { name: 'Navigation API',         meta: 'WS · v1.0'     },
    { name: 'TravelAssistantMCP',     meta: 'MCP · v1.0.0'  },
  ];
  var TOTAL = SAMPLES.length;

  function dismiss() {
    try { sessionStorage.setItem(STORAGE_KEY, '1'); } catch (_) {}
    overlay.style.display = 'none';
  }

  function show(panelEl) {
    [introPanel, deployPanel, donePanel].forEach(function (p) { p.classList.remove('active'); });
    panelEl.classList.add('active');
  }

  function runAnimation(onComplete) {
    var done = 0;

    function loadCard(s, checked) {
      curNameEl.textContent = s.name;
      curMetaEl.textContent = s.meta;
      curSpinner.style.display = checked ? 'none' : 'block';
      curCheck.style.display   = checked ? 'inline-flex' : 'none';
    }

    function fadeIn() {
      curCard.style.transition = 'none';
      curCard.style.opacity    = '0';
      curCard.style.transform  = 'translateY(6px)';
      curCard.offsetHeight; // flush layout so transition:none takes effect
      curCard.style.transition = '';
      curCard.style.opacity    = '1';
      curCard.style.transform  = 'none';
    }

    function fadeOut(cb) {
      curCard.style.opacity   = '0';
      curCard.style.transform = 'translateY(-6px)';
      setTimeout(cb, 280);
    }

    function tick(i) {
      if (i >= TOTAL) { onComplete && onComplete(); return; }
      var s = SAMPLES[i];
      loadCard(s, false);
      fadeIn();
      setTimeout(function () {
        done++;
        loadCard(s, true);
        progressFill.style.width  = Math.round(done / TOTAL * 100) + '%';
        progressLabel.textContent = done + ' of ' + TOTAL + ' deployed. This only takes a moment.';
        setTimeout(function () {
          if (i + 1 >= TOTAL) { onComplete && onComplete(); return; }
          fadeOut(function () { tick(i + 1); });
        }, 480);
      }, 760);
    }

    tick(0);
  }

  function runDeploy() {
    show(deployPanel);
    deployBtn.disabled = true;

    var animDone = false;
    var fetchDone = false;
    var fetchResult = null;

    function tryFinish() {
      if (!animDone || !fetchDone) return;
      var deployed = fetchResult ? fetchResult.deployed : 0;
      var skipped  = fetchResult ? fetchResult.skipped  : 0;
      var failed   = fetchResult ? fetchResult.failed   : 0;
      var detail = [];
      if (deployed > 0) detail.push(deployed + ' deployed');
      if (skipped  > 0) detail.push(skipped  + ' already existed');
      if (failed   > 0) detail.push(failed   + ' failed');
      doneMsgEl.textContent     = 'Sample APIs are ready in your portal. Taking you to your API catalog…';
      doneDetailEl.textContent  = detail.join(' · ');
      show(donePanel);
      setTimeout(function () {
        try { sessionStorage.setItem(STORAGE_KEY, '1'); } catch (_) {}
        window.location.href = BASE_URL + '/apis';
      }, 2000);
    }

    // Kick off animation
    runAnimation(function () { animDone = true; tryFinish(); });

    // Kick off real deployment
    var csrfToken = '';
    try {
      var m = document.cookie.match(/(?:^|;\s*)XSRF-TOKEN=([^;]+)/);
      if (m) csrfToken = decodeURIComponent(m[1]);
    } catch (_) {}

    fetch(SEED_URL, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'X-CSRF-Token': csrfToken, 'Content-Type': 'application/json' },
    })
    .then(function (r) { return r.json(); })
    .then(function (data) { fetchResult = data; })
    .catch(function (err) {
      console.warn('Sample seed fetch error:', err);
      fetchResult = { deployed: 0, skipped: 0, failed: 0 };
    })
    .finally(function () { fetchDone = true; tryFinish(); });
  }

  // Server already decides whether to render this overlay at all (demo mode + not yet
  // seeded — see showOnboarding in orgContentController.js). This session-scoped check
  // just avoids re-showing it after a "skip" click within the same browser session.
  try {
    if (sessionStorage.getItem(STORAGE_KEY)) {
      overlay.style.display = 'none';
      return;
    }
  } catch (_) {}

  deployBtn.addEventListener('click', runDeploy);
  skipBtn.addEventListener('click', dismiss);
  overlay.addEventListener('click', function (e) { if (e.target === overlay) dismiss(); });
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && overlay.style.display !== 'none') dismiss();
  });
}());
