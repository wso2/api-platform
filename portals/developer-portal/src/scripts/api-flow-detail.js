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

/**
 * Show a temporary notification toast
 */
function showNotification(message, type = 'info') {
    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    const content = document.createElement('div');
    content.className = 'notification-content';
    const icon = document.createElement('i');
    icon.classList.add('bi', type === 'success' ? 'bi-check-circle' : 'bi-exclamation-circle');
    const span = document.createElement('span');
    span.textContent = message;
    content.append(icon, span);
    notification.appendChild(content);

    document.body.appendChild(notification);

    setTimeout(() => {
        notification.classList.add('show');
    }, 10);

    setTimeout(() => {
        notification.classList.remove('show');
        setTimeout(() => {
            notification.remove();
        }, 300);
    }, 3000);
}

/**
 * Render markdown content into the markdownViewer container
 */
function renderMarkdownPreview(content) {
    const container = document.getElementById('markdownViewer');
    if (!container) return;

    if (!content) {
        container.innerHTML = '<p class="af-md-empty">No workflow description available.</p>';
        return;
    }

    if (window.marked && window.DOMPurify) {
        marked.setOptions({ breaks: true, gfm: true });
        const rawHtml = marked.parse(content);
        container.innerHTML = DOMPurify.sanitize(rawHtml);
    } else {
        const pre = document.createElement('pre');
        pre.className = 'af-md-fallback';
        pre.textContent = content;
        container.replaceChildren(pre);
    }
}

/**
 * Initialise the Arazzo UI viewer with the given spec content
 */
function initArazzoUI(specContent) {
    const viewerContainer = document.getElementById('arazzoViewer');
    if (!viewerContainer) return;

    if (!specContent) {
        viewerContainer.innerHTML = '<p style="padding: 2rem; text-align: center; color: #999;">No specification content available</p>';
        return;
    }

    try {
        if (window.ArazzoUI) {
            window.ArazzoUI({
                dom_id: '#arazzoViewer',
                document: specContent,
                view: 'split'
            });
        } else {
            throw new Error('Arazzo UI library not loaded.');
        }
    } catch (error) {
        console.error('Error initializing Arazzo UI:', error);
        const wrapper = document.createElement('div');
        wrapper.style.cssText = 'padding:2rem;text-align:center;color:#e74c3c;';
        const title = document.createElement('p');
        title.innerHTML = '<strong>Error loading Arazzo UI</strong>';
        const message = document.createElement('p');
        message.style.cssText = 'font-size:0.9rem;color:#999;';
        message.textContent = error.message;
        wrapper.appendChild(title);
        wrapper.appendChild(message);
        viewerContainer.replaceChildren(wrapper);
    }
}

/**
 * Wire up prompt modal action buttons and initialise the spec viewer
 */
document.addEventListener('DOMContentLoaded', function() {
    const btnCopyPrompt = document.getElementById('btnCopyPrompt');
    if (btnCopyPrompt) {
        btnCopyPrompt.addEventListener('click', function() {
            const promptEl = document.getElementById('modalAgentPromptText');
            if (!promptEl) { showNotification('Prompt not available', 'error'); return; }
            const promptText = promptEl.textContent;
            navigator.clipboard.writeText(promptText).then(() => {
                const icon = btnCopyPrompt.querySelector('i');
                icon.className = 'bi bi-check-lg';
                setTimeout(() => { icon.className = 'bi bi-copy'; }, 2000);
            }).catch(() => showNotification('Failed to copy prompt', 'error'));
        });
    }

    const btnDownloadPrompt = document.getElementById('btnDownloadPrompt');
    if (btnDownloadPrompt) {
        btnDownloadPrompt.addEventListener('click', function() {
            const promptEl = document.getElementById('modalAgentPromptText');
            if (!promptEl) return;
            const promptText = promptEl.textContent;
            const blob = new Blob([promptText], { type: 'text/plain' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = (document.querySelector('main').dataset.flowHandle || 'workflow') + '.txt';
            a.click();
            URL.revokeObjectURL(url);
        });
    }

    const btnRunClaude = document.getElementById('btnRunClaude');
    if (btnRunClaude) {
        btnRunClaude.addEventListener('click', function() {
            const promptEl = document.getElementById('modalAgentPromptText');
            if (!promptEl) return;
            const requestId = Math.floor(Math.random() * 1e10);
            const prompt = promptEl.textContent.replace(/(https?:\/\/[^\s]+\.md)(?!\?)/g, '$1?request-id=' + requestId);
            window.open('https://claude.ai/new?q=' + encodeURIComponent(prompt), '_blank');
        });
    }

    const specContent = document.getElementById('specContent').value.trim();
    const contentType = document.querySelector('main').dataset.flowContentType;

    if (contentType === 'MD') {
        renderMarkdownPreview(specContent);
    } else {
        initArazzoUI(specContent);
    }
});
