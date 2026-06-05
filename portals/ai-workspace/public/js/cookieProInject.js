/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

var __sentryEnv = (window.__RUNTIME_CONFIG__ && window.__RUNTIME_CONFIG__.VITE_SENTRY_ENV) || '';
var __isDevEnvironment = __sentryEnv === 'DEV';
var __isStageEnvironment = __sentryEnv === 'STAGE';
var __isProdEnvironment = __sentryEnv === 'PROD';
var __hideCookieFloatingStyleId = 'cookiepro-hide-floating-style';

function hasCookie(name) {
  return new RegExp('(?:^|;\\s*)' + name + '=').test(document.cookie || '');
}

function shouldHideCookieFloatingButton() {
  return hasCookie('OptanonConsent') || hasCookie('OptanonAlertBoxClosed');
}

function addCookieFloatingHideStyle() {
  if (document.getElementById(__hideCookieFloatingStyleId)) {
    return;
  }

  var styleTag = document.createElement('style');
  styleTag.id = __hideCookieFloatingStyleId;
  styleTag.textContent =
    '#ot-sdk-btn-floating, .ot-floating-button, button.ot-sdk-show-settings {' +
    'display: none !important;' +
    'visibility: hidden !important;' +
    'opacity: 0 !important;' +
    'pointer-events: none !important;' +
    '}';
  document.head.appendChild(styleTag);
}

function hideCookieFloatingButtonIfNeeded() {
  if (!shouldHideCookieFloatingButton()) {
    return;
  }

  addCookieFloatingHideStyle();

  var floatingButton = document.getElementById('ot-sdk-btn-floating');
  if (floatingButton) {
    floatingButton.style.display = 'none';
  }
}

function setupCookieFloatingButtonHider() {
  hideCookieFloatingButtonIfNeeded();

  window.addEventListener('OneTrustGroupsUpdated', hideCookieFloatingButtonIfNeeded);

  var observer = new MutationObserver(function () {
    hideCookieFloatingButtonIfNeeded();
  });

  observer.observe(document.documentElement, {
    childList: true,
    subtree: true,
  });
}

function appendCookieProSnippet() {
  var cookie_pro_script_tag = document.createElement('script');
  cookie_pro_script_tag.setAttribute('src', 'https://cookie-cdn.cookiepro.com/scripttemplates/otSDKStub.js');
  cookie_pro_script_tag.setAttribute('type', 'text/javascript');
  if (__isDevEnvironment) {
    cookie_pro_script_tag.setAttribute('data-domain-script', '0195891c-b6ea-7e3f-bf0f-915545ca7dc9-test');
  } else if (__isStageEnvironment) {
    cookie_pro_script_tag.setAttribute('data-domain-script', '0195891c-b6ea-7e3f-bf0f-915545ca7dc9-test');
  } else if (__isProdEnvironment) {
    cookie_pro_script_tag.setAttribute('data-domain-script', '0195891c-b6ea-7e3f-bf0f-915545ca7dc9');
  }
  cookie_pro_script_tag.setAttribute('charset', 'UTF-8');
  document.head.appendChild(cookie_pro_script_tag);
}

(function () {
  if (__isDevEnvironment || __isProdEnvironment || __isStageEnvironment) {
    setupCookieFloatingButtonHider();
    appendCookieProSnippet();
  }
})();
