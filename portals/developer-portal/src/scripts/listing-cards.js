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

    (function(){
      function findClosest(el, selector){
        while(el && el !== document){
          if(el.matches && el.matches(selector)) return el;
          el = el.parentNode;
        }
        return null;
      }

      document.addEventListener('click', function(e){
        var modalTrigger = findClosest(e.target, '[data-modal]');
        if(modalTrigger){
          e.preventDefault();
          if(modalTrigger.classList.contains('is-readonly') || modalTrigger.getAttribute('aria-disabled') === 'true'){
            return;
          }
          if(typeof loadModal === 'function'){
            loadModal(modalTrigger.getAttribute('data-modal'));
          } else {
            var id = modalTrigger.getAttribute('data-modal');
            var el = document.getElementById(id);
            if(el) {
              el.style.display = 'flex';
              document.body.classList.add('modal-open');
              if(typeof prepareSubscriptionModal === 'function') {
                try { prepareSubscriptionModal(id); } catch(err) { /* noop */ }
              }
            }
          }
          return;
        }

        var nav = findClosest(e.target, '[data-href]');
        if(nav){
          var href = nav.getAttribute('data-href');
          if(href){ window.location.href = href; }
        }
      }, false);
    })();
