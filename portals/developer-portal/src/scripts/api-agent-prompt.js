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

let _agentMdUrl = '';

function buildAgentPrompt(mdUrl) {
    return `Please fetch the API details from the following URL:\n\n${mdUrl}\n\nOnce you've read it, provide a structured summary covering:\n- What this API does\n- Authentication mechanisms and how to obtain credentials\n- Available endpoints\n- Main resources and key fields in their responses\n\nThen ask me what I'd like to do with it, with a few sample options to choose from.`;
}

function copyAgentPrompt() {
    const text = document.getElementById('apiAgentPromptText').textContent;
    navigator.clipboard.writeText(text).then(() => {
        const icon = document.getElementById('apiBtnCopyPrompt').querySelector('i');
        icon.className = 'bi bi-check2';
        setTimeout(() => { icon.className = 'bi bi-copy'; }, 2000);
    });
}

function downloadAgentPrompt(apiName) {
    const text = document.getElementById('apiAgentPromptText').textContent;
    const blob = new Blob([text], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = apiName + '.txt';
    a.click();
    URL.revokeObjectURL(url);
}

function toggleApiRunDropdown(event) {
    event.stopPropagation();
    document.getElementById('apiRunDropdownMenu').classList.toggle('show');
}

function runApiPromptInClaude() {
    const text = _agentMdUrl
        ? buildAgentPrompt(_agentMdUrl + '?request-id=' + Math.floor(Math.random() * 1e10))
        : document.getElementById('apiAgentPromptText').textContent;
    window.open('https://claude.ai/new?q=' + encodeURIComponent(text), '_blank');
}

function runApiPromptInChatGPT() {
    const text = document.getElementById('apiAgentPromptText').textContent;
    window.open('https://chatgpt.com/?prompt=' + encodeURIComponent(text), '_blank');
    document.getElementById('apiRunDropdownMenu').classList.remove('show');
}

function addRipple(btn, e) {
    const existing = btn.querySelector('.btn-ripple');
    if (existing) existing.remove();
    const rect = btn.getBoundingClientRect();
    const size = Math.max(rect.width, rect.height) * 2;
    const x = e.clientX - rect.left - size / 2;
    const y = e.clientY - rect.top - size / 2;
    const ripple = document.createElement('span');
    ripple.className = 'btn-ripple';
    Object.assign(ripple.style, {
        position: 'absolute', borderRadius: '50%',
        width: size + 'px', height: size + 'px',
        left: x + 'px', top: y + 'px',
        background: 'rgba(255,255,255,0.35)',
        transform: 'scale(0)', pointerEvents: 'none',
        animation: 'btn-ripple-anim 0.55s ease-out forwards',
    });
    btn.appendChild(ripple);
    ripple.addEventListener('animationend', () => ripple.remove());
}

document.addEventListener('DOMContentLoaded', () => {
    let apiName = '';

    const aiBtn = document.querySelector('.btn-icon-ai');
    if (aiBtn) aiBtn.addEventListener('click', (e) => addRipple(aiBtn, e));

    const dataEl = document.getElementById('apiAgentData');
    if (dataEl) {
        try {
            const data = JSON.parse(dataEl.textContent);
            const mdUrl = window.location.origin + data.baseUrl + '/api/' + data.apiHandle + '.md';
            _agentMdUrl = mdUrl;
            apiName = data.apiHandle;

            const modalEl = document.getElementById('apiAgentPromptModal');
            if (modalEl) {
                modalEl.addEventListener('show.bs.modal', function () {
                    document.getElementById('apiAgentPromptText').textContent = buildAgentPrompt(mdUrl);
                });
            }
        } catch (e) {
            console.error('Failed to parse apiAgentData:', e);
        }
    }

    const apiBtnCopyPrompt = document.getElementById('apiBtnCopyPrompt');
    if (apiBtnCopyPrompt) apiBtnCopyPrompt.addEventListener('click', copyAgentPrompt);

    const apiBtnDownloadPrompt = document.getElementById('apiBtnDownloadPrompt');
    if (apiBtnDownloadPrompt) apiBtnDownloadPrompt.addEventListener('click', () => downloadAgentPrompt(apiName));

    const apiBtnRunPrompt = document.getElementById('apiBtnRunPrompt');
    if (apiBtnRunPrompt) apiBtnRunPrompt.addEventListener('click', toggleApiRunDropdown);

    const apiBtnRunClaude = document.getElementById('apiBtnRunClaude');
    if (apiBtnRunClaude) apiBtnRunClaude.addEventListener('click', runApiPromptInClaude);

    const apiBtnRunChatGPT = document.getElementById('apiBtnRunChatGPT');
    if (apiBtnRunChatGPT) apiBtnRunChatGPT.addEventListener('click', runApiPromptInChatGPT);

    document.addEventListener('click', () => {
        const menu = document.getElementById('apiRunDropdownMenu');
        if (menu) menu.classList.remove('show');
    });
});
