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
  function init() {
    if (typeof window.particlesJS === 'undefined') return;
    var el = document.getElementById('hero-particles');
    if (!el) return;
    window.particlesJS('hero-particles', {
      particles: {
        number: { value: 64, density: { enable: true, value_area: 900 } },
        color: { value: ['var(--white)', '#f9a04b', '#5cd1ff'] },
        shape: { type: 'circle' },
        opacity: { value: 0.5, random: true },
        size: { value: 2.6, random: true },
        line_linked: { enable: true, distance: 150, color: '#9fb6cc', opacity: 0.18, width: 1 },
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
