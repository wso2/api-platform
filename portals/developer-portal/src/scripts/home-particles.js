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

/* Particles init — runs after DOM ready */
(function () {
  /* Canvas fillStyle cannot read CSS custom properties, so resolve the theme
     tokens to concrete hex values at runtime (via a probe element the browser
     fully computes, including color-mix()) and feed those to particles.js.
     Editing the seeds in main.css re-themes the particles on next load. */
  function resolveHex(varName, fallback) {
    try {
      var probe = document.createElement('span');
      probe.style.cssText = 'color:var(' + varName + ',' + fallback + ');position:absolute;visibility:hidden';
      document.body.appendChild(probe);
      var m = getComputedStyle(probe).color.match(/\d+(?:\.\d+)?/g);
      probe.parentNode.removeChild(probe);
      if (!m || m.length < 3) return fallback;
      return '#' + m.slice(0, 3).map(function (n) {
        return ('0' + Math.round(parseFloat(n)).toString(16)).slice(-2);
      }).join('');
    } catch (e) {
      return fallback;
    }
  }

  function init() {
    if (typeof window.particlesJS === 'undefined') return;
    var el = document.getElementById('hero-particles');
    if (!el) return;

    var dotColors = [
      resolveHex('--white', '#ffffff'),
      resolveHex('--accent-light', '#f9a04b'),
      resolveHex('--on-dark-pill', '#5cd1ff'),
    ];
    var linkColor = resolveHex('--primary-light', '#9fb6cc');

    window.particlesJS('hero-particles', {
      particles: {
        number: { value: 64, density: { enable: true, value_area: 900 } },
        color: { value: dotColors },
        shape: { type: 'circle' },
        opacity: { value: 0.5, random: true },
        size: { value: 2.6, random: true },
        line_linked: { enable: true, distance: 150, color: linkColor, opacity: 0.18, width: 1 },
        move: { enable: true, speed: 1.1, direction: 'none', out_mode: 'out' },
      },
      interactivity: {
        detect_on: 'canvas',
        events: { onhover: { enable: true, mode: 'grab' }, resize: true },
        modes: { grab: { distance: 160, line_linked: { opacity: 0.35 } } },
      },
      retina_detect: true,
    });
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
  /* Retry once after a short delay in case particles.js loads after this script */
  setTimeout(init, 400);
})();
