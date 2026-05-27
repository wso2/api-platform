/* eslint-disable no-undef */
/* eslint-disable no-unused-vars */

// ─────────────────────────────────────────────
// Data Initialization
// ─────────────────────────────────────────────

let apiFlowsData = [];
let currentOrgID = '';
let currentViewName = '';
let csrfToken = '';
let arazoEditor = null;
let showSelectedOnly = false;
const activeApiFilters = new Set();
let currentWorkflowPath = 'upload';
let currentContentType = 'ARAZZO';
let createPathFormat = 'arazzo'; // 'arazzo' | 'md'
let currentStep = 1;

function activateTab(activeBtn) {
    const tabMap = [
        { btnId: 'llms-tab-btn',      paneId: 'llmsTabContent' },
        { btnId: 'workflows-tab-btn', paneId: 'workflowsTabContent' },
    ];
    tabMap.forEach(({ btnId, paneId }) => {
        const btn  = document.getElementById(btnId);
        const pane = document.getElementById(paneId);
        const isActive = btn === activeBtn;
        if (btn)  { btn.classList.toggle('active', isActive); btn.setAttribute('aria-selected', String(isActive)); }
        if (pane) pane.style.display = isActive ? '' : 'none';
    });
    if (activeBtn?.id === 'workflows-tab-btn') arazoEditor?.refresh();
}

function initializeApiFlowsData() {
    try {
        const dataContainer = document.getElementById('apiFlowsDataContainer');
        if (dataContainer) {
            apiFlowsData = JSON.parse(dataContainer.textContent) || [];
        }
    } catch (e) {
        console.error('Failed to parse apiFlowsData:', e);
        apiFlowsData = [];
    }

    try {
        const contextContainer = document.getElementById('apiFlowsContextData');
        if (contextContainer) {
            const context = JSON.parse(contextContainer.textContent);
            currentOrgID = context.orgID || '';
            currentViewName = context.viewName || '';
            csrfToken = context.csrfToken || '';
        }
    } catch (e) {
        console.error('Failed to parse apiFlows context:', e);
    }
}

// ─────────────────────────────────────────────
// Handle Generation
// ─────────────────────────────────────────────

function generateHandle(name) {
    return name
        .toLowerCase()
        .trim()
        .replace(/[^\w\s-]/g, '')
        .replace(/\s+/g, '-')
        .replace(/-+/g, '-')
        .substring(0, 100);
}

document.addEventListener('DOMContentLoaded', function () {
    initializeApiFlowsData();

    const listSection = document.getElementById('apiFlowList');
    const formSection = document.getElementById('apiFlowForm');
    const createBtn = document.getElementById('createApiFlowBtn');
    const createBtnEmpty = document.getElementById('createApiFlowBtnEmpty');

    function showForm() {
        listSection.style.display = 'none';
        formSection.style.display = 'block';
    }

    function showList() {
        listSection.style.display = 'block';
        formSection.style.display = 'none';
        resetApiFlowForm();
        // Switch to the API Workflows tab when returning to the list.
        const wfBtn = document.getElementById('workflows-tab-btn');
        if (wfBtn) activateTab(wfBtn);
    }

    function handleCreateClick() {
        resetApiFlowForm();
        document.getElementById('apiFlowFormTitle').textContent = 'Create API Flow';
        document.getElementById('editingApiFlowId').value = '';
        updatePromptFromForm();
        showForm();
    }

    createBtn?.addEventListener('click', handleCreateClick);
    createBtnEmpty?.addEventListener('click', handleCreateClick);

    document.getElementById('cancelApiFlowBtn')?.addEventListener('click', showList);
    document.getElementById('cancelApiFlowBtn2')?.addEventListener('click', showList);

    // Debounced prompt update on name/description change
    let promptDebounce;
    const debouncePromptUpdate = () => {
        clearTimeout(promptDebounce);
        promptDebounce = setTimeout(updatePromptFromForm, 600);
    };
    document.getElementById('apiFlowName')?.addEventListener('input', debouncePromptUpdate);
    document.getElementById('apiFlowDescription')?.addEventListener('input', debouncePromptUpdate);

    // Form action buttons
    document.getElementById('regeneratePromptBtn')?.addEventListener('click', regenerateAgentPrompt);
    document.getElementById('copyFieldPromptBtn')?.addEventListener('click', copyFieldPrompt);

    // Agent visibility radios
    document.querySelectorAll('input[name="apiFlowAgentVisibility"]').forEach(radio => {
        radio.addEventListener('change', () => {
            syncAgentPromptTab(radio.value === 'HIDDEN' && radio.checked);
            if (radio.value === 'VISIBLE' && radio.checked) {
                document.querySelectorAll('.api-flow-api-checkbox:checked').forEach(cb => {
                    if (cb.dataset.agentVisibility !== 'VISIBLE') cb.checked = false;
                });
            }
            renderApiCards(document.getElementById('apiCardSearch')?.value.trim() || '');
        });
    });

    document.getElementById('changeVisibilityBtn')?.addEventListener('click', () => {
        goToStep(1);
        setTimeout(() => {
            document.getElementById('agentVisibleCard')?.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }, 80);
    });

    // Unified editor copy button
    document.getElementById('copySpecBtn')?.addEventListener('click', () => {
        if (createPathFormat === 'markdown') copyMarkdown(); else copyArazzoSpec();
    });

    // Markdown word count
    document.getElementById('markdownContent')?.addEventListener('input', () => {
        updateMarkdownWordCount();
        if (createPathFormat === 'markdown') updateEditorFooter('markdown');
    });

    // Save buttons
    document.getElementById('saveApiFlowBtn')?.addEventListener('click', function() {
        saveApiFlow(currentOrgID, currentViewName, this.dataset.status);
    });
    document.getElementById('saveDraftBtn')?.addEventListener('click', function() {
        saveApiFlow(currentOrgID, currentViewName, this.dataset.status);
    });

    // List view action buttons
    document.querySelectorAll('.api-flow-view-prompt-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.preventDefault();
            openPromptModal(btn.dataset.apiFlowId);
        });
    });

    document.querySelectorAll('.api-flow-edit-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.preventDefault();
            openEditApiFlow(btn.dataset.apiFlowId);
        });
    });

    document.querySelectorAll('.api-flow-delete-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.preventDefault();
            openDeleteApiFlowModal(currentOrgID, currentViewName, btn.dataset.apiFlowId);
        });
    });

    document.getElementById('copyPromptBtn')?.addEventListener('click', copyPrompt);
    document.getElementById('downloadPromptBtn')?.addEventListener('click', downloadPrompt);

    document.getElementById('runInClaudeBtn')?.addEventListener('click', function() {
        const requestId = Math.floor(Math.random() * 1e10);
        const prompt = document.getElementById('agentPromptContent').textContent.replace(/(https?:\/\/[^\s]+\.md)(?!\?)/g, '$1?request-id=' + requestId);
        window.open('https://claude.ai/new?q=' + encodeURIComponent(prompt), '_blank');
    });

    // Section summaries
    document.getElementById('apiFlowName')?.addEventListener('input', updateSectionSummaries);
    document.querySelectorAll('input[name="apiFlowVisibility"]').forEach(r => r.addEventListener('change', updateSectionSummaries));
    document.querySelectorAll('input[name="apiFlowAgentVisibility"]').forEach(r => r.addEventListener('change', updateSectionSummaries));

    initApiCardPicker();
    initCodeMirrorEditor();
    initSectionCollapse();
    initWorkflowPathChooser();
    initUploadZone();
    initCreateFormatToggle();
    initBringBack();
    initCreatePathButtons();
    initWizard();
    initAccessMatrix();

    // Keep access matrix in sync when radios change by other means
    document.querySelectorAll('input[name="apiFlowVisibility"]').forEach(r => r.addEventListener('change', syncAccessMatrixFromRadios));
    document.querySelectorAll('input[name="apiFlowAgentVisibility"]').forEach(r => r.addEventListener('change', syncAccessMatrixFromRadios));
});

// ─────────────────────────────────────────────
// Workflow Path Chooser
// ─────────────────────────────────────────────

function initWorkflowPathChooser() {
    document.querySelectorAll('.af-path-card').forEach(card => {
        card.addEventListener('click', () => switchWorkflowPath(card.dataset.path));
        card.addEventListener('keydown', e => {
            if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); switchWorkflowPath(card.dataset.path); }
        });
    });
}

function switchWorkflowPath(path) {
    currentWorkflowPath = path;

    document.querySelectorAll('.af-path-card').forEach(card => {
        const isActive = card.dataset.path === path;
        card.classList.toggle('af-path-card--active', isActive);
        card.setAttribute('aria-pressed', String(isActive));
    });

    const uploadContent = document.getElementById('uploadPathContent');
    const createContent = document.getElementById('createPathContent');
    uploadContent?.classList.toggle('d-none', path !== 'upload');
    createContent?.classList.toggle('d-none', path !== 'create');

    // Actions (Generate with Claude, VS Code, Copy Prompt) are only relevant in the create path.
    document.querySelector('.af-rp-actions')?.classList.toggle('d-none', path === 'upload');

    // In create path: honour the format toggle (createPathFormat owns currentContentType).
    // In upload path: show whichever editor matches the uploaded file type (or hide both).
    if (path === 'create') {
        document.getElementById('specFormatTabs')?.classList.remove('d-none');
        switchCreateFormat(createPathFormat);
    } else {
        showUploadedContentEditor();
    }

    updateSectionSummaries();
}

// ─────────────────────────────────────────────
// Create Path Format Toggle (Arazzo / Markdown)
// ─────────────────────────────────────────────

function initCreateFormatToggle() {
    document.querySelectorAll('.af-create-format-btn').forEach(btn => {
        btn.addEventListener('click', () => switchCreateFormat(btn.dataset.format));
    });
}

function switchCreateFormat(format) {
    createPathFormat = format;
    currentContentType = format === 'markdown' ? 'MD' : 'ARAZZO';

    // Update toggle button active state
    document.querySelectorAll('.af-create-format-btn').forEach(btn => {
        btn.classList.toggle('af-create-format-btn--active', btn.dataset.format === format);
    });

    // Show/hide the right editor pane
    document.getElementById('apiFlowDefinitionWrapper')?.classList.toggle('d-none', format !== 'arazzo');
    document.getElementById('markdownContentWrapper')?.classList.toggle('d-none', format !== 'markdown');
    if (format === 'arazzo') setTimeout(() => arazoEditor?.refresh(), 50);

    document.getElementById('specEditorCard')?.classList.remove('d-none');

    // Update footer copy button label for the active format
    updateEditorFooter(format);

    // "Open in VS Code" only makes sense for Arazzo
    document.getElementById('openInVSCodeBtn')?.classList.toggle('d-none', format !== 'arazzo');

    // Update Generate Template button label
    const label = document.getElementById('generateTemplateBtnLabel');
    if (label) label.textContent = format === 'markdown' ? 'Generate Template' : 'Generate Template';

    updateSectionSummaries();
}

// ─────────────────────────────────────────────
// Bring it Back (re-import edited file)
// ─────────────────────────────────────────────

function initBringBack() {
    const btn = document.getElementById('bringBackBtn');
    const fileInput = document.getElementById('bringBackFileInput');
    if (!btn || !fileInput) return;

    btn.addEventListener('click', () => fileInput.click());
    fileInput.addEventListener('change', e => {
        const file = e.target.files[0];
        if (file) handleBringBackFile(file);
        e.target.value = '';
    });
}

function showBringBackFeedback(msg, type) {
    const el = document.getElementById('bringBackFeedback');
    if (!el) return;
    el.textContent = msg;
    el.className = 'af-bring-back-feedback' + (type ? ` af-bring-back-feedback--${type}` : '');
    if (type === 'success') {
        setTimeout(() => {
            if (el.textContent === msg) {
                el.textContent = '';
                el.className = 'af-bring-back-feedback';
            }
        }, 3000);
    }
}

function handleBringBackFile(file) {
    const ext = file.name.substring(file.name.lastIndexOf('.')).toLowerCase();
    const valid = ['.yaml', '.yml', '.json', '.md'];
    if (!valid.includes(ext)) {
        showBringBackFeedback(`Use a ${valid.join(', ')} file.`, 'error');
        return;
    }

    const reader = new FileReader();
    reader.onerror = () => showBringBackFeedback("Couldn't read that file. Try again.", 'error');
    reader.onload = e => {
        const content = e.target.result;
        if (!content.trim()) {
            showBringBackFeedback('That file is empty.', 'error');
            return;
        }

        const isMarkdown = ext === '.md';
        const targetFormat = isMarkdown ? 'markdown' : 'arazzo';
        const targetContentType = isMarkdown ? 'MD' : 'ARAZZO';

        const editingApiFlowId = document.getElementById('editingApiFlowId')?.value || '';
        if (editingApiFlowId) {
            if (currentContentType !== targetContentType) {
                showBringBackFeedback(
                    `Cannot import: this flow uses ${currentContentType === 'MD' ? 'Markdown' : 'Arazzo'} format, but the uploaded file is ${targetContentType === 'MD' ? 'Markdown' : 'Arazzo'}.`,
                    'error'
                );
                return;
            }
            // Types match — load content without mutating currentContentType
        } else {
            switchCreateFormat(targetFormat);
        }

        if (isMarkdown) {
            const mdField = document.getElementById('markdownContent');
            if (mdField) mdField.value = content;
            const filenameEl = document.querySelector('#markdownContentWrapper .af-editor-filename');
            if (filenameEl) filenameEl.textContent = file.name;
            const lineCount = content.split('\n').length;
            const mdWordCount = document.getElementById('mdWordCount');
            if (mdWordCount) mdWordCount.textContent = `${lineCount} line${lineCount === 1 ? '' : 's'}`;
            updateEditorFooter('markdown');
        } else {
            if (arazoEditor) {
                arazoEditor.setValue(content);
                setTimeout(() => arazoEditor.refresh(), 50);
            } else {
                const field = document.getElementById('apiFlowDefinition');
                if (field) field.value = content;
            }
            const filenameEl = document.querySelector('#apiFlowDefinitionWrapper .af-editor-filename');
            if (filenameEl) filenameEl.textContent = file.name;
            updateArazzoEditorUI(content);
        }

        showBringBackFeedback(`Template updated from ${file.name}`, 'success');
        updateSectionSummaries();
    };
    reader.readAsText(file);
}

// ─────────────────────────────────────────────
// Upload Zone
// ─────────────────────────────────────────────

function initUploadZone() {
    const dropZone = document.getElementById('arazzoDropZone');
    const fileInput = document.getElementById('arazoFileInput');

    if (!dropZone) return;

    // Click to browse
    dropZone.addEventListener('click', () => fileInput?.click());
    document.getElementById('uploadBrowseBtn')?.addEventListener('click', (e) => {
        e.stopPropagation();
        fileInput?.click();
    });
    dropZone.addEventListener('keydown', e => {
        if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); fileInput?.click(); }
    });

    // Drag events
    dropZone.addEventListener('dragover', e => {
        e.preventDefault();
        dropZone.classList.add('af-upload-zone--dragover');
    });
    dropZone.addEventListener('dragleave', e => {
        if (!dropZone.contains(e.relatedTarget)) {
            dropZone.classList.remove('af-upload-zone--dragover');
        }
    });
    dropZone.addEventListener('drop', e => {
        e.preventDefault();
        dropZone.classList.remove('af-upload-zone--dragover');
        const file = e.dataTransfer?.files[0];
        if (file) processArazzoFile(file);
    });

    // File input change
    fileInput?.addEventListener('change', e => {
        const file = e.target.files[0];
        if (file) processArazzoFile(file);
        e.target.value = '';
    });

    // Remove file
    document.getElementById('removeUploadBtn')?.addEventListener('click', clearUploadedFile);

    // Source Descriptions accordion toggle
    document.getElementById('sdValidationToggle')?.addEventListener('click', () => {
        const body = document.getElementById('sdValidationItems');
        const toggle = document.getElementById('sdValidationToggle');
        if (!body || !toggle) return;
        const isExpanded = toggle.getAttribute('aria-expanded') === 'true';
        toggle.setAttribute('aria-expanded', String(!isExpanded));
        body.classList.toggle('af-sd-collapsed', isExpanded);
    });
}

function processArazzoFile(file) {
    const ext = file.name.substring(file.name.lastIndexOf('.')).toLowerCase();
    const validArazzo = ['.yaml', '.yml', '.json'];
    const validMd = ['.md'];

    if (!validArazzo.includes(ext) && !validMd.includes(ext)) {
        showAlert('Please upload a .yaml, .yml, .json, or .md file', 'error');
        return;
    }

    const isMarkdown = validMd.includes(ext);
    const reader = new FileReader();
    reader.onload = async (e) => {
        const content = e.target.result;

        // Show file strip
        document.getElementById('uploadFileName').textContent = file.name;
        document.getElementById('uploadValidationPanel')?.classList.remove('d-none');
        document.getElementById('arazzoDropZone')?.classList.add('d-none');

        if (isMarkdown) {
            await handleMarkdownUpload(content);
        } else {
            await handleArazzoUpload(content);
        }

        updateSectionSummaries();
    };
    reader.readAsText(file);
}

async function handleArazzoUpload(content) {
    currentContentType = 'ARAZZO';

    // Load into CodeMirror
    if (arazoEditor) {
        arazoEditor.setValue(content);
    } else {
        const field = document.getElementById('apiFlowDefinition');
        if (field) field.value = content;
    }

    // Clear markdown
    document.getElementById('markdownContent').value = '';

    // Update type badge
    setUploadTypeBadge('arazzo');

    // Show/hide panels
    document.getElementById('sdValidationContainer')?.classList.remove('d-none');
    document.getElementById('mdSummaryPanel')?.classList.add('d-none');
    showUploadedContentEditor();

    await validateAndRenderSourceDescriptions(content);
    updateCopyArazzoBtn(content.trim().length > 0);
}

async function handleMarkdownUpload(content) {
    currentContentType = 'MD';

    // Load into markdown textarea
    const mdField = document.getElementById('markdownContent');
    if (mdField) mdField.value = content;

    // Clear arazzo
    if (arazoEditor) arazoEditor.setValue('');
    const arazoField = document.getElementById('apiFlowDefinition');
    if (arazoField) arazoField.value = '';

    // Update type badge
    setUploadTypeBadge('markdown');

    // Show/hide panels
    document.getElementById('sdValidationContainer')?.classList.add('d-none');

    const wordCount = content.trim().split(/\s+/).filter(Boolean).length;
    const lineCount = content.split('\n').length;
    const mdSummary = document.getElementById('mdSummaryPanel');
    const mdSummaryText = document.getElementById('mdSummaryText');
    if (mdSummary && mdSummaryText) {
        mdSummaryText.textContent = `${wordCount.toLocaleString()} words · ${lineCount} lines`;
        mdSummary.classList.remove('d-none');
    }

    showUploadedContentEditor();
    updateMarkdownWordCount();
}

function setUploadTypeBadge(type) {
    const badge = document.getElementById('uploadTypeBadge');
    if (!badge) return;
    if (type === 'arazzo') {
        badge.textContent = 'Arazzo';
        badge.className = 'af-upload-type-badge af-upload-type-badge--arazzo';
    } else {
        badge.textContent = 'Markdown';
        badge.className = 'af-upload-type-badge af-upload-type-badge--md';
    }
}

function showUploadedContentEditor() {
    const arazoWrapper = document.getElementById('apiFlowDefinitionWrapper');
    const mdWrapper = document.getElementById('markdownContentWrapper');
    const card = document.getElementById('specEditorCard');
    const hasArazzo = arazoEditor
        ? arazoEditor.getValue().trim().length > 0
        : (document.getElementById('apiFlowDefinition')?.value?.trim().length > 0);
    const hasMd = (document.getElementById('markdownContent')?.value?.trim().length > 0);

    // Hide format tabs in upload mode — they're create-only
    document.getElementById('specFormatTabs')?.classList.add('d-none');

    if (currentContentType === 'MD' && hasMd) {
        arazoWrapper?.classList.add('d-none');
        mdWrapper?.classList.remove('d-none');
        card?.classList.remove('d-none');
        updateEditorFooter('markdown');
    } else if (currentContentType === 'ARAZZO' && hasArazzo) {
        mdWrapper?.classList.add('d-none');
        arazoWrapper?.classList.remove('d-none');
        card?.classList.remove('d-none');
        setTimeout(() => arazoEditor?.refresh(), 50);
        updateEditorFooter('arazzo');
    } else {
        // No file uploaded yet — show the editor shell (with its built-in empty state)
        mdWrapper?.classList.add('d-none');
        arazoWrapper?.classList.remove('d-none');
        card?.classList.remove('d-none');
    }
}

function clearUploadedFile() {
    if (arazoEditor) arazoEditor.setValue('');
    const arazoField = document.getElementById('apiFlowDefinition');
    if (arazoField) arazoField.value = '';
    const mdField = document.getElementById('markdownContent');
    if (mdField) mdField.value = '';

    currentContentType = 'ARAZZO';

    document.getElementById('uploadValidationPanel')?.classList.add('d-none');
    document.getElementById('apiFlowDefinitionWrapper')?.classList.add('d-none');
    document.getElementById('markdownContentWrapper')?.classList.add('d-none');
    document.getElementById('specEditorCard')?.classList.add('d-none');
    document.getElementById('arazzoDropZone')?.classList.remove('d-none');
    document.getElementById('sdValidationContainer')?.classList.add('d-none');
    document.getElementById('mdSummaryPanel')?.classList.add('d-none');
    if (document.getElementById('sdValidationItems')) document.getElementById('sdValidationItems').innerHTML = '';
    if (document.getElementById('sdValidationHint')) document.getElementById('sdValidationHint').textContent = '';
    updateSectionSummaries();
}

// ─────────────────────────────────────────────
// Source Description Validation
// ─────────────────────────────────────────────

async function validateAndRenderSourceDescriptions(specContent) {
    const hintEl = document.getElementById('sdValidationHint');
    const itemsEl = document.getElementById('sdValidationItems');
    if (!hintEl || !itemsEl) return;

    hintEl.textContent = '';
    itemsEl.innerHTML = '<div class="af-sd-item af-sd-item--loading"><span class="af-sd-checking-spinner"></span><span class="text-muted small">Validating source descriptions…</span></div>';

    // Always expand while checking so the loading state is visible.
    const toggleEl = document.getElementById('sdValidationToggle');
    if (toggleEl) toggleEl.setAttribute('aria-expanded', 'true');
    itemsEl.classList.remove('af-sd-collapsed');

    let spec = null;
    try {
        spec = JSON.parse(specContent);
    } catch {
        if (window.jsyaml) {
            try { spec = window.jsyaml.load(specContent); } catch { /* invalid */ }
        }
    }

    if (!spec) {
        itemsEl.innerHTML = '<div class="af-sd-item"><span class="af-sd-status af-sd-status--invalid"><i class="bi bi-x-circle-fill me-1"></i>Parse error</span><span class="text-muted small ms-2">Could not parse file as Arazzo YAML/JSON</span></div>';
        return;
    }

    const sourceDeps = spec.sourceDescriptions;
    if (!Array.isArray(sourceDeps) || sourceDeps.length === 0) {
        itemsEl.innerHTML = '<div class="af-sd-item"><span class="text-muted small fst-italic">No source descriptions found in this spec</span></div>';
        hintEl.textContent = '(none)';
        return;
    }

    // Show loading state per item
    itemsEl.innerHTML = sourceDeps.map(sd => renderSdItem(sd.name, sd.url, sd.type, 'checking')).join('');

    // Validate all URLs in parallel
    const results = await Promise.all(sourceDeps.map(sd => checkSourceDescriptionUrl(sd)));

    // Render final results
    itemsEl.innerHTML = results.map(r => renderSdItem(r.name, r.url, r.type, r.status, r.statusCode)).join('');

    const validCount = results.filter(r => r.status === 'valid').length;
    const total = results.length;
    hintEl.textContent = `— ${validCount}/${total} valid`;

    // Collapse automatically when everything is valid; expand if any issue found.
    const allValid = total > 0 && validCount === total;
    const toggle = document.getElementById('sdValidationToggle');
    if (toggle) toggle.setAttribute('aria-expanded', String(!allValid));
    itemsEl.classList.toggle('af-sd-collapsed', allValid);
}

async function checkSourceDescriptionUrl(sd) {
    const result = { name: sd.name || 'unnamed', url: sd.url, type: sd.type, status: 'checking' };
    if (!sd.url) { result.status = 'missing'; return result; }
    try {
        const resp = await fetch(sd.url, {
            method: 'GET',
            credentials: 'same-origin',
            headers: { 'Accept': 'application/json, application/yaml, text/yaml, */*' }
        });
        result.statusCode = resp.status;
        if (!resp.ok) {
            result.status = 'invalid';
            return result;
        }
        const text = await resp.text();
        result.status = isValidApiSpec(text) ? 'valid' : 'not-a-spec';
    } catch {
        result.status = 'unreachable';
    }
    return result;
}

function isValidApiSpec(text) {
    if (!text || !text.trim()) return false;
    // Try JSON first
    try {
        const obj = JSON.parse(text);
        return !!(obj && (obj.openapi || obj.swagger || obj.asyncapi || obj.arazzo));
    } catch { /* not JSON */ }
    // Try YAML
    if (window.jsyaml) {
        try {
            const obj = window.jsyaml.load(text);
            return !!(obj && (obj.openapi || obj.swagger || obj.asyncapi || obj.arazzo));
        } catch { /* not YAML */ }
    }
    // Last resort: raw text heuristic (avoids HTML pages slipping through)
    return /^\s*(openapi|swagger|asyncapi|arazzo)\s*:/im.test(text);
}

function renderSdItem(name, url, type, status, statusCode) {
    const statusMap = {
        checking:     { cls: 'af-sd-status--checking',     icon: 'bi-hourglass-split',         label: 'Checking…' },
        valid:        { cls: 'af-sd-status--valid',         icon: 'bi-check-circle-fill',       label: statusCode ? `${statusCode} · Valid spec` : 'Valid spec' },
        invalid:      { cls: 'af-sd-status--invalid',       icon: 'bi-x-circle-fill',           label: statusCode ? `${statusCode} Error` : 'Error' },
        'not-a-spec': { cls: 'af-sd-status--not-a-spec',    icon: 'bi-file-earmark-x',          label: 'Not an API spec' },
        unreachable:  { cls: 'af-sd-status--unreachable',   icon: 'bi-wifi-off',                label: 'Unreachable' },
        missing:      { cls: 'af-sd-status--invalid',       icon: 'bi-exclamation-circle-fill', label: 'No URL' },
    };
    const s = statusMap[status] || statusMap.checking;
    const typeBadge = type ? `<span class="api-flow-type-pill ms-1">${sanitizeInput(type)}</span>` : '';
    const urlText = url ? `<span class="af-sd-item-url">${sanitizeInput(url)}</span>` : '<span class="af-sd-item-url text-muted">No URL defined</span>';
    return `
        <div class="af-sd-item">
            <span class="af-sd-status ${s.cls}"><i class="bi ${s.icon} me-1"></i>${s.label}</span>
            <div class="af-sd-item-meta">
                <span class="af-sd-item-name">${sanitizeInput(name)}${typeBadge}</span>
                ${urlText}
            </div>
        </div>`;
}

// ─────────────────────────────────────────────
// Create Path Buttons
// ─────────────────────────────────────────────

function initCreatePathButtons() {
    document.getElementById('generateWithClaudeBtn')?.addEventListener('click', generateWithClaude);
    document.getElementById('copyPromptSpecBtn')?.addEventListener('click', copySpecPrompt);
    document.getElementById('openInVSCodeBtn')?.addEventListener('click', openInVSCode);
}

function updateGenerateButtonsState() {
    const count = document.querySelectorAll('.api-flow-api-checkbox:checked').length;
    const hasAPIs = count > 0;
    const tip = document.getElementById('specActionsTip');

    ['generateWithClaudeBtn', 'copyPromptSpecBtn', 'openInVSCodeBtn'].forEach(id => {
        const btn = document.getElementById(id);
        if (!btn) return;
        btn.disabled = !hasAPIs;
        if (hasAPIs) {
            btn.removeAttribute('aria-describedby');
        } else {
            btn.setAttribute('aria-describedby', 'specActionsTip');
        }
    });

    if (tip) tip.textContent = hasAPIs ? '' : 'Select at least one API to enable';
}

function copySpecPrompt() {
    const apis = getSelectedAPIs();
    if (apis.length === 0) return;

    const pathParts = window.location.pathname.split('/');
    const orgHandle = pathParts[1] || '';
    const viewName = pathParts[3] || 'default';
    const name = document.getElementById('apiFlowName')?.value?.trim() || '';
    const description = document.getElementById('apiFlowDescription')?.value?.trim() || '';

    const apiContext = apis.map(a => {
        const url = `${window.location.origin}/${orgHandle}/views/${viewName}/api/${a.apiHandle}/docs/specification.json`;
        return `- **${a.apiName}** (${a.apiType || 'REST'}): ${a.apiDescription || 'No description provided'}\n  OpenAPI spec: ${url}`;
    }).join('\n');

    const contextBlock = [
        name ? `**Workflow name:** ${name}` : null,
        description ? `**Initial description:** ${description}` : null,
    ].filter(Boolean).join('\n');

    const isMarkdown = createPathFormat === 'markdown';
    const prompt = isMarkdown
        ? buildMarkdownPrompt(contextBlock, apiContext)
        : buildArazzoPrompt(contextBlock, apiContext);

    navigator.clipboard.writeText(prompt).then(() => showAlert('Prompt copied to clipboard', 'success'));
}

function buildArazzoPrompt(contextBlock, apiContext) {
    return `You are helping a developer define an API workflow that will be saved to their developer portal.

${contextBlock ? `Here is some context they have already provided:\n${contextBlock}\n` : ''}**Available APIs on their portal:**
${apiContext}

Before doing anything else, fetch and read each OpenAPI spec URL above. Use the specs to identify the available API operations, required inputs and parameters, response shapes, and documented error codes — this will drive the steps and inputs you generate.

Your job is to have a short conversation to understand exactly what the workflow should do, then produce a complete Arazzo 1.0.0 YAML spec.

**Step 1 — Ask the developer to describe the workflow in natural language.** Ask them: what triggers it, what each API call should do in sequence, what data flows between steps, and what the expected outcome is. If any detail is unclear, ask a follow-up question before proceeding.

**Step 2 — Once you have enough detail, generate a complete Arazzo 1.0.0 YAML spec that:**
1. Uses the \`arazzo: '1.0.0'\` header
2. Includes \`sourceDescriptions\` with the OpenAPI spec URLs listed above
3. Defines a \`workflows\` array with a single workflow and well-structured \`steps\`
4. Each step includes: \`stepId\`, \`description\`, \`operationPath\`, \`successCriteria\`, \`outputs\`, \`onSuccess\`, \`onFailure\`
5. Includes \`inputs\` with required fields and \`outputs\` at the workflow level

Output the raw YAML only — no prose around it.

**Step 3 — After outputting the YAML, tell the developer:** "Your Arazzo spec is ready. Please copy the YAML above, switch back to your developer portal tab, and paste it into the workflow editor."`;
}

function buildMarkdownPrompt(contextBlock, apiContext) {
    return `You are helping a developer describe an API workflow in plain language. The result will be saved as a Markdown document in their developer portal so that AI agents can understand and execute the workflow.

${contextBlock ? `Here is some context they have already provided:\n${contextBlock}\n` : ''}**APIs available on their portal:**
${apiContext}

Before doing anything else, fetch and read each OpenAPI spec URL above. Use the specs to identify the available API operations, required inputs and parameters, response shapes, and documented error codes — this will inform the steps and error handling you describe.

Your job is to have a short conversation to understand the workflow, then produce a clear Markdown document describing it.

**Step 1 — Ask the developer to describe the workflow in natural language.** Ask them: what triggers it, which APIs are called and in what order, what data is passed between steps, and what the final outcome is. Ask follow-up questions if anything is unclear.

**Step 2 — Once you have enough detail, write a well-structured Markdown document that:**
1. Opens with a concise summary of what the workflow does and why
2. Lists the APIs involved with a one-line explanation of each
3. Describes every step in plain English — which API is called, what data is sent, what comes back, and why that step is needed
4. Explains what a successful run produces for the caller
5. Covers error scenarios in a simple table: what can go wrong and what happens

Use Markdown formatting (headers, numbered lists, tables, bold for emphasis). Write for a developer who needs to understand the flow at a glance. Output the raw Markdown only — no prose around it.

**Step 3 — After outputting the Markdown, tell the developer:** "Your workflow description is ready. Please copy the Markdown above, switch back to your developer portal tab, and paste it into the workflow editor."`;
}

function generateWithClaude() {
    if (createPathFormat === 'markdown') {
        generateMarkdownWithClaude();
    } else {
        generateArazzoWithClaude();
    }
}

function generateArazzoWithClaude() {
    const { contextBlock, apiContext } = buildPromptContext();
    const prompt = buildArazzoPrompt(contextBlock, apiContext);
    window.open('https://claude.ai/new?q=' + encodeURIComponent(prompt), '_blank');
}

function generateMarkdownWithClaude() {
    const { contextBlock, apiContext } = buildPromptContext();
    const prompt = buildMarkdownPrompt(contextBlock, apiContext);
    window.open('https://claude.ai/new?q=' + encodeURIComponent(prompt), '_blank');
}

function buildPromptContext() {
    const name = document.getElementById('apiFlowName')?.value?.trim() || '';
    const description = document.getElementById('apiFlowDescription')?.value?.trim() || '';
    const apis = getSelectedAPIs();
    const pathParts = window.location.pathname.split('/');
    const orgHandle = pathParts[1] || '';
    const viewName = pathParts[3] || 'default';

    const apiContext = apis.length > 0
        ? apis.map(a => {
            const url = `${window.location.origin}/${orgHandle}/views/${viewName}/api/${a.apiHandle}/docs/specification.json`;
            return `- **${a.apiName}** (${a.apiType || 'REST'}): ${a.apiDescription || 'No description provided'}\n  OpenAPI spec: ${url}`;
        }).join('\n')
        : '*(No APIs pre-selected — select the required APIs from the Associated APIs section above. For APIs published in the developer portal, the Source Description will be auto-generated.)*';

    const contextBlock = [
        name ? `**Workflow name:** ${name}` : null,
        description ? `**Initial description:** ${description}` : null,
    ].filter(Boolean).join('\n');

    return { contextBlock, apiContext };
}

async function openInVSCode() {
    if (createPathFormat === 'markdown') return; // button is hidden in markdown mode

    generateArazzoSpec();

    const content = arazoEditor ? arazoEditor.getValue() : (document.getElementById('apiFlowDefinition')?.value || '');
    if (!content.trim()) {
        showAlert('Generate a template first or select some APIs', 'warning');
        return;
    }

    const name = document.getElementById('apiFlowName')?.value?.trim() || 'workflow';
    const filename = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') + '.arazzo.yaml';

    const btn = document.getElementById('openInVSCodeBtn');
    const originalHtml = btn?.innerHTML;
    if (btn) btn.innerHTML = '<span class="af-sd-checking-spinner"></span> Opening…';

    try {
        const res = await fetch('/devportal/temp-arazzo-file', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
            body: JSON.stringify({ content, filename }),
            credentials: 'same-origin'
        });

        if (!res.ok) throw new Error('Server error');
        const { path: filePath } = await res.json();

        // Use vscode:// URI to open the file directly in VS Code
        // filePath is absolute (starts with /), so omit the extra slash after "file"
        const vscodeUri = `vscode://file${filePath}`;

        await new Promise((resolve) => {
            let handled = false;

            const cleanup = () => {
                clearTimeout(timer);
                document.removeEventListener('visibilitychange', onVisibilityChange);
                window.removeEventListener('pagehide', onPageHide);
            };

            const onVisibilityChange = () => {
                if (document.hidden) {
                    handled = true;
                    cleanup();
                    resolve();
                }
            };

            const onPageHide = () => {
                handled = true;
                cleanup();
                resolve();
            };

            document.addEventListener('visibilitychange', onVisibilityChange);
            window.addEventListener('pagehide', onPageHide);

            // Attempt to open via hidden iframe to avoid navigating away
            const iframe = document.createElement('iframe');
            iframe.style.display = 'none';
            iframe.src = vscodeUri;
            document.body.appendChild(iframe);
            setTimeout(() => document.body.removeChild(iframe), 2000);

            const timer = setTimeout(() => {
                cleanup();
                if (!handled) {
                    // VS Code protocol handler not available — offer download fallback
                    const blob = new Blob([content], { type: 'application/yaml' });
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.href = url;
                    a.download = filename;
                    document.body.appendChild(a);
                    a.click();
                    document.body.removeChild(a);
                    URL.revokeObjectURL(url);
                    showAlert(
                        `VS Code does not appear to be installed or the protocol handler is not registered — "${filename}" downloaded instead.`,
                        'warning'
                    );
                }
                resolve();
            }, 1200);
        });

    } catch (err) {
        // Fallback: download the file
        const blob = new Blob([content], { type: 'application/yaml' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
        showAlert(`VS Code open failed (${err?.message || err}) — "${filename}" downloaded instead`, 'warning');
    } finally {
        if (btn && originalHtml) btn.innerHTML = originalHtml;
    }
}

// ─────────────────────────────────────────────
// Form state helpers
// ─────────────────────────────────────────────

function resetApiFlowForm() {
    document.getElementById('editingApiFlowId').value = '';
    const nameField = document.getElementById('apiFlowName');
    if (nameField) { nameField.value = ''; nameField.readOnly = false; nameField.classList.remove('af-field-readonly'); }

    document.getElementById('apiFlowDescription').value = '';
    document.getElementById('apiFlowDefinition').value = '';
    document.getElementById('markdownContent').value = '';
    document.getElementById('agentPromptField').value = '';
    currentContentType = 'ARAZZO';
    createPathFormat = 'arazzo';

    if (arazoEditor) arazoEditor.setValue('');

    const visibilityPublicBtn = document.getElementById('visibilityPublic');
    if (visibilityPublicBtn) visibilityPublicBtn.checked = true;
    const agentVisibilityVisibleBtn = document.getElementById('agentVisibilityVisible');
    if (agentVisibilityVisibleBtn) agentVisibilityVisibleBtn.checked = true;
    syncAgentPromptTab(false);

    showSelectedOnly = false;
    activeApiFilters.clear();
    document.querySelectorAll('.af-api-filter-check').forEach(cb => { cb.checked = false; });
    updateApiFilterBadge();
    setPickerSelection([]);
    setSaveButtonMode('create');
    updateSectionSummaries();
    expandAllSections();

    // Re-enable format toggle for new flows.
    document.querySelectorAll('.af-create-format-btn').forEach(btn => {
        btn.disabled = false;
        btn.title = '';
    });

    // Reset to upload path
    clearUploadedFile();
    switchWorkflowPath('upload');
    switchCreateFormat('arazzo');

    // Reset wizard to step 1
    currentStep = 1;
    if (typeof updateWizardUI === 'function') updateWizardUI();
}

function syncAgentPromptTab(isHidden) {
    const hiddenIcon = document.getElementById('tabVisualHiddenIcon');
    const banner = document.getElementById('agentHiddenBanner');
    const toolbar = document.querySelector('.af-prompt-action-toolbar');
    const promptField = document.getElementById('agentPromptField');
    const ready3 = document.getElementById('afReady3');
    const agentHiddenNotice = document.getElementById('afAgentHiddenNotice');

    if (isHidden) {
        hiddenIcon?.classList.remove('d-none');
        if (banner) {
            banner.classList.remove('d-none', 'af-banner-fade-out');
            banner.classList.add('af-banner-fade-in');
        }
        toolbar?.classList.add('d-none');
        promptField?.classList.add('d-none');
        ready3?.classList.add('d-none');
        agentHiddenNotice?.classList.remove('d-none');
    } else {
        hiddenIcon?.classList.add('d-none');
        if (banner && !banner.classList.contains('d-none')) {
            banner.classList.remove('af-banner-fade-in');
            banner.classList.add('af-banner-fade-out');
            setTimeout(() => {
                banner.classList.add('d-none');
                banner.classList.remove('af-banner-fade-out');
            }, 150);
        }
        toolbar?.classList.remove('d-none');
        promptField?.classList.remove('d-none');
        ready3?.classList.remove('d-none');
        agentHiddenNotice?.classList.add('d-none');
    }
}

function getSelectedAPIs() {
    return [...document.querySelectorAll('.api-flow-api-checkbox:checked')].map(cb => ({
        API_ID: cb.value,        apiId: cb.value,
        API_NAME: cb.dataset.apiName,       apiName: cb.dataset.apiName,
        API_HANDLE: cb.dataset.apiHandle,   apiHandle: cb.dataset.apiHandle,
        API_DESCRIPTION: cb.dataset.apiDescription, apiDescription: cb.dataset.apiDescription,
        API_TYPE: cb.dataset.apiType,       apiType: cb.dataset.apiType,
        PRODUCTION_URL: cb.dataset.productionUrl,   productionUrl: cb.dataset.productionUrl
    }));
}

// ─────────────────────────────────────────────
// API Card Picker
// ─────────────────────────────────────────────

function updateApiFilterBadge() {
    const badge = document.getElementById('apiFilterBadge');
    const btn = document.getElementById('apiFilterDropdownBtn');
    if (!badge) return;
    const count = activeApiFilters.size + (showSelectedOnly ? 1 : 0);
    if (count > 0) {
        badge.textContent = count;
        badge.classList.remove('d-none');
        btn?.classList.add('af-api-filter-toggle--active');
    } else {
        badge.classList.add('d-none');
        btn?.classList.remove('af-api-filter-toggle--active');
    }
}

function initApiCardPicker() {
    const searchEl = document.getElementById('apiCardSearch');
    if (!searchEl) return;
    searchEl.addEventListener('input', () => renderApiCards(searchEl.value.trim()));

    // Filter dropdown toggle
    const dropdownBtn = document.getElementById('apiFilterDropdownBtn');
    const filterMenu = document.getElementById('apiFilterMenu');
    if (dropdownBtn && filterMenu) {
        dropdownBtn.addEventListener('click', e => {
            e.stopPropagation();
            const open = filterMenu.classList.toggle('af-api-filter-menu--open');
            dropdownBtn.setAttribute('aria-expanded', open);
        });
        document.addEventListener('click', e => {
            if (!filterMenu.contains(e.target) && e.target !== dropdownBtn) {
                filterMenu.classList.remove('af-api-filter-menu--open');
                dropdownBtn.setAttribute('aria-expanded', false);
            }
        });
    }

    // Filter checkboxes
    document.querySelectorAll('.af-api-filter-check').forEach(cb => {
        cb.addEventListener('change', () => {
            const key = cb.dataset.afFilter;
            if (key === 'selectedOnly') {
                showSelectedOnly = cb.checked;
            } else if (cb.checked) {
                activeApiFilters.add(key);
            } else {
                activeApiFilters.delete(key);
            }
            updateApiFilterBadge();
            renderApiCards(searchEl.value.trim());
        });
    });

    renderApiCards('');
}

function renderApiCards(query) {
    const grid = document.getElementById('apiCardGrid');
    if (!grid) return;

    const checkboxes = [...document.querySelectorAll('.api-flow-api-checkbox')];
    const q = query.toLowerCase();
    const FILTER_FNS = {
        aiReady:      cb => cb.dataset.agentVisibility === 'VISIBLE',
        aiRestricted: cb => cb.dataset.agentVisibility !== 'VISIBLE',
    };
    const filtered = checkboxes.filter(cb => {
        if (showSelectedOnly && !cb.checked) return false;
        if (activeApiFilters.size > 0 && ![...activeApiFilters].every(f => FILTER_FNS[f]?.(cb))) return false;
        if (!q) return true;
        return (cb.dataset.apiName || '').toLowerCase().includes(q)
            || (cb.dataset.apiDescription || '').toLowerCase().includes(q)
            || (cb.dataset.apiType || '').toLowerCase().includes(q);
    });

    if (filtered.length === 0) {
        grid.innerHTML = '<p class="text-muted small fst-italic mb-0">No matching APIs</p>';
        updateApiSelectedCount();
        return;
    }

    const workflowVisibleToAgents = document.querySelector('input[name="apiFlowAgentVisibility"]:checked')?.value === 'VISIBLE';

    const pathParts = window.location.pathname.split('/');
    const orgHandle = pathParts[1] || '';
    const viewName  = pathParts[3] || 'default';

    grid.innerHTML = filtered.map(cb => {
        const isSelected = cb.checked;
        const isAgentReady = cb.dataset.agentVisibility === 'VISIBLE';
        const isDisabled = workflowVisibleToAgents && !isAgentReady;
        const agentBadge = isAgentReady
            ? `<span class="af-api-agent-badge af-api-agent-badge--ready" title="AI ready"><i class="bi bi-robot"></i></span>`
            : `<span class="af-api-agent-badge af-api-agent-badge--not-ready" title="Not AI ready"><i class="bi bi-robot"></i></span>`;
        const disabledTooltip = isDisabled
            ? `<span class="af-api-card-tooltip">This API is not AI ready and cannot be selected for an AI-visible workflow</span>`
            : '';
        const docsUrl = `/${orgHandle}/views/${viewName}/api/${cb.dataset.apiHandle}.md`;
        const extLink = isAgentReady
            ? `<a class="af-api-card-ext-link" href="${docsUrl}" target="_blank" rel="noopener"
                  title="Open API docs" aria-label="Open ${sanitizeInput(cb.dataset.apiName)} docs in new tab">
                   <i class="bi bi-box-arrow-up-right"></i>
               </a>`
            : '';
        return `
            <div class="af-api-card${isSelected ? ' af-api-card--selected' : ''}${isDisabled ? ' af-api-card--disabled' : ''}"
                 data-api-id="${cb.value}" role="button" tabindex="${isDisabled ? -1 : 0}"
                 aria-pressed="${isSelected}" aria-disabled="${isDisabled}">
                ${disabledTooltip}
                <div class="af-api-card-check">
                    <i class="bi ${isSelected ? 'bi-check-circle-fill' : 'bi-circle'}"></i>
                </div>
                <div class="af-api-card-body">
                    <div class="d-flex align-items-center gap-2 mb-1">
                        <span class="fw-semibold small af-api-card-name" title="${sanitizeInput(cb.dataset.apiName)}">${sanitizeInput(cb.dataset.apiName)}</span>
                        <span class="api-flow-type-pill flex-shrink-0">${sanitizeInput(cb.dataset.apiType || '')}</span>
                        ${agentBadge}
                    </div>
                </div>
                ${extLink}
            </div>
        `;
    }).join('');

    grid.querySelectorAll('.af-api-card').forEach(card => {
        function toggle() {
            const cb = document.querySelector(`.api-flow-api-checkbox[value="${card.dataset.apiId}"]`);
            if (!cb || card.classList.contains('af-api-card--disabled')) return;
            cb.checked = !cb.checked;
            renderApiCards(document.getElementById('apiCardSearch')?.value.trim() || '');
            updatePromptFromForm();
            if (currentWorkflowPath === 'create') generateArazzoSpec();
        }
        card.addEventListener('click', toggle);
        card.addEventListener('keydown', e => {
            if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggle(); }
        });
        // Prevent the ext-link click from toggling selection.
        card.querySelector('.af-api-card-ext-link')?.addEventListener('click', e => e.stopPropagation());
    });

    updateApiSelectedCount();
    updateApiChips();
    updateSectionSummaries();
}

function updateApiSelectedCount() {
    updateGenerateButtonsState();
}

function setPickerSelection(apiIds) {
    document.querySelectorAll('.api-flow-api-checkbox').forEach(cb => {
        cb.checked = apiIds.includes(cb.value);
    });
    renderApiCards(document.getElementById('apiCardSearch')?.value.trim() || '');
}

// ─────────────────────────────────────────────
// Agent Prompt Generation
// ─────────────────────────────────────────────

async function updatePromptFromForm() {
    const name = document.getElementById('apiFlowName')?.value?.trim() || '';
    const description = document.getElementById('apiFlowDescription')?.value?.trim() || '';
    const apis = getSelectedAPIs();
    const pathParts = window.location.pathname.split('/');
    const orgName = pathParts[1] || '';
    const viewName = pathParts[3] || 'default';
    const editingId = document.getElementById('editingApiFlowId')?.value || '';
    const editingFlow = editingId ? (window.apiFlowsData || []).find(f => String(f.apiFlowId) === String(editingId)) : null;
    const handle = editingFlow?.handle || generateHandle(name);

    try {
        const response = await fetch(`/${orgName}/views/${viewName}/api-flows/generate-prompt`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
            body: JSON.stringify({ name, description, apis, orgName, viewName, handle }),
            credentials: 'same-origin'
        });
        if (response.ok) {
            const data = await response.json();
            const promptField = document.getElementById('agentPromptField');
            if (promptField) promptField.value = data.agentPrompt || '';
            updateStep3Readiness();
        }
    } catch (error) {
        console.error('Error generating prompt:', error);
    }
}

function regenerateAgentPrompt() {
    updatePromptFromForm();
}


function updateWorkflowMdPreview() {
    const el = document.getElementById('afWorkflowMdPreview');
    if (!el) return;

    const name = document.getElementById('apiFlowName')?.value?.trim() || '';
    const desc = document.getElementById('apiFlowDescription')?.value?.trim() || '';
    const apis = getSelectedAPIs();
    const isMarkdown = createPathFormat === 'markdown';

    if (!name && !desc) {
        el.textContent = 'Fill in name and description to see a preview.';
        return;
    }

    const pathParts = window.location.pathname.split('/');
    const orgHandle = pathParts[1] || '';
    const viewName = pathParts[3] || 'default';
    const editingId = document.getElementById('editingApiFlowId')?.value;
    const editingFlow = editingId ? (window.apiFlowsData || []).find(f => String(f.apiFlowId) === editingId) : null;
    const handle = editingFlow?.handle || generateHandle(name);
    const flowStatus = (editingFlow?.status || 'PUBLISHED').toUpperCase();

    let md = '';
    md += `# ${name}\n\n`;
    md += `**Status:** ${flowStatus}\n\n`;
    if (desc) md += `**Description:** ${desc}\n`;

    if (apis.length > 0) {
        md += `\n## Sources\n\n`;
        apis.forEach(a => {
            const url = `/${orgHandle}/views/${viewName}/api/${a.apiHandle}/docs/specification.json`;
            md += `- **${a.apiName}** — [API Documentation](${url})\n`;
        });
        md += `\n> Refer to each source for base URLs, endpoints, security schemes, and any additional documentation needed to execute this workflow.\n`;
        md += `\n## Authentication\n\n`;
        md += `To determine the authentication method required for each source:\n\n`;
        md += `- **API Documentation sources**: Check the source's documentation page for an **API Key** section or an **OAuth2** section.\n`;
        md += `- **OpenAPI Spec sources**: Read the \`securitySchemes\` section of the spec to determine the authentication method.\n`;
    }

    if (isMarkdown) {
        const workflowDesc = document.getElementById('markdownContent')?.value?.trim() || '';
        md += `\n## Workflow Description\n\n`;
        md += workflowDesc || '_No workflow description defined yet._';
        md += '\n';
    } else {
        const spec = arazoEditor ? arazoEditor.getValue().trim() : (document.getElementById('apiFlowDefinition')?.value?.trim() || '');
        md += `\n## API Flow Specification\n\n`;
        md += `[arazzo.json](/${orgHandle}/views/${viewName}/api-workflows/${handle}/arazzo.json)\n\n`;
        if (spec) {
            md += '``````\n' + spec + '\n``````\n'; // Use 6 backticks so rendering holds even if the spec contains up to 5
        } else {
            md += '_No specification defined yet._\n';
        }
    }

    el.textContent = md;
}

// ─────────────────────────────────────────────
// Save button state
// ─────────────────────────────────────────────

function setSaveButtonMode(mode, currentStatus) {
    const mainBtn = document.getElementById('saveApiFlowBtn');
    const badge = document.getElementById('editingFlowStatusBadge');
    if (!mainBtn) return;

    if (mode === 'edit') {
        mainBtn.innerHTML = '<i class="bi bi-check2 me-1"></i> Update Flow';
        if (badge && currentStatus) {
            badge.textContent = currentStatus;
            badge.className = `api-flow-status-badge api-flow-status-${currentStatus.toLowerCase()}`;
        }
    } else {
        mainBtn.innerHTML = '<i class="bi bi-send me-1"></i> Publish Flow';
        if (badge) badge.className = 'api-flow-status-badge d-none';
    }
}

// ─────────────────────────────────────────────
// Save (Create or Update)
// ─────────────────────────────────────────────

async function saveApiFlow(orgID, viewName, status) {
    if (arazoEditor) {
        document.getElementById('apiFlowDefinition').value = arazoEditor.getValue();
    }

    const name = document.getElementById('apiFlowName').value.trim();
    const description = document.getElementById('apiFlowDescription').value.trim();
    const agentPrompt = document.getElementById('agentPromptField').value.trim();
    const contentType = currentContentType || 'ARAZZO';
    const apiFlowDefinition = contentType === 'ARAZZO' ? document.getElementById('apiFlowDefinition').value.trim() : '';
    const markdownContent = contentType === 'MD' ? document.getElementById('markdownContent').value.trim() : '';
    const apiFlowId = document.getElementById('editingApiFlowId').value;
    const agentVisibility = document.querySelector('input[name="apiFlowAgentVisibility"]:checked')?.value || 'VISIBLE';
    let valid = true;
    const fieldsToValidate = [
        ['apiFlowName', name],
        ['apiFlowDescription', description],
    ];
    if (agentVisibility !== 'HIDDEN') fieldsToValidate.push(['agentPromptField', agentPrompt]);
    if (contentType === 'ARAZZO') fieldsToValidate.push(['apiFlowDefinition', apiFlowDefinition]);
    if (contentType === 'MD') fieldsToValidate.push(['markdownContent', markdownContent]);

    fieldsToValidate.forEach(([id, val]) => {
        if (id === 'apiFlowDefinition' && arazoEditor) {
            const host = document.getElementById('arazoEditorHost');
            const feedback = document.getElementById('apiFlowDefinitionInvalid');
            if (!val) {
                host?.classList.add('af-cm-invalid');
                if (feedback) feedback.style.display = 'block';
                valid = false;
            } else {
                host?.classList.remove('af-cm-invalid');
                if (feedback) feedback.style.display = 'none';
            }
        } else {
            const el = document.getElementById(id);
            if (!val) {
                el?.classList.add('is-invalid');
                valid = false;
            } else {
                el?.classList.remove('is-invalid');
            }
        }
    });
    if (!valid) return;

    const visibility = document.querySelector('input[name="apiFlowVisibility"]:checked')?.value || 'PUBLIC';
    const handle = generateHandle(name);
    const payload = { name, handle, description, agentPrompt, status, visibility, agentVisibility, contentType, apiFlowDefinition, markdownContent };
    const isEdit = !!apiFlowId;
    const url = isEdit
        ? `/devportal/organizations/${orgID}/views/${viewName}/api-flows/${apiFlowId}`
        : `/devportal/organizations/${orgID}/views/${viewName}/api-flows`;
    const method = isEdit ? 'PUT' : 'POST';

    const groupBtns = document.querySelectorAll('#saveApiFlowGroup button');
    const mainBtn = document.getElementById('saveApiFlowBtn');
    groupBtns.forEach(b => b.disabled = true);
    mainBtn.innerHTML = '<span class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span> Saving…';

    try {
        const response = await fetch(url, {
            method,
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
            body: JSON.stringify(payload),
            credentials: 'same-origin'
        });
        if (response.ok) {
            window.location.href = window.location.pathname + '#apiflows';
            window.location.reload();
        } else {
            const err = await response.json().catch(() => ({ message: 'Save failed' }));
            showAlert(err.message || 'Save failed', 'error');
            groupBtns.forEach(b => b.disabled = false);
            setSaveButtonMode(isEdit ? 'edit' : 'create');
        }
    } catch (err) {
        showAlert(err.message || 'Network error', 'error');
        groupBtns.forEach(b => b.disabled = false);
        setSaveButtonMode(isEdit ? 'edit' : 'create');
    }
}

// ─────────────────────────────────────────────
// Delete
// ─────────────────────────────────────────────

function openDeleteApiFlowModal(orgID, viewName, apiFlowId) {
    const flow = (apiFlowsData || []).find(f => String(f.apiFlowId) === String(apiFlowId));
    const flowName = flow?.name || 'API Flow';

    document.getElementById('deleteApiFlowModalTitle').textContent = 'Delete API Flow';
    const messageEl = document.getElementById('deleteApiFlowModalMessage');
    messageEl.textContent = `Are you sure you want to delete "${flowName}"? This action cannot be undone.`;

    const confirmBtn = document.getElementById('deleteApiFlowConfirmBtn');
    confirmBtn.disabled = false;
    confirmBtn.innerHTML = 'Confirm';
    confirmBtn.onclick = () => deleteApiFlow(orgID, viewName, apiFlowId);
    const modal = new bootstrap.Modal(document.getElementById('deleteApiFlowModal'));
    modal.show();
}

async function deleteApiFlow(orgID, viewName, apiFlowId) {
    const confirmBtn = document.getElementById('deleteApiFlowConfirmBtn');
    confirmBtn.disabled = true;
    confirmBtn.style.backgroundColor = 'var(--danger-color)';
    confirmBtn.innerHTML = '<span class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span> Deleting…';

    const resetBtn = () => {
        confirmBtn.disabled = false;
        confirmBtn.style.backgroundColor = '';
        confirmBtn.innerHTML = 'Confirm';
    };

    try {
        const response = await fetch(`/devportal/organizations/${orgID}/views/${viewName}/api-flows/${apiFlowId}`, {
            method: 'DELETE',
            headers: { 'X-CSRF-Token': csrfToken },
            credentials: 'same-origin'
        });
        if (response.ok) {
            window.location.reload();
        } else {
            resetBtn();
            showAlert('Failed to delete API Flow', 'error');
        }
    } catch (error) {
        resetBtn();
        showAlert(`Failed to delete API Flow: ${error.message}`, 'error');
    }
}

// ─────────────────────────────────────────────
// Edit — pre-fill form from existing data
// ─────────────────────────────────────────────

function inferApiIdsFromContent(data) {
    const checkboxes = [...document.querySelectorAll('.api-flow-api-checkbox')];
    if (!checkboxes.length) return [];
    const contentType = data.contentType || 'ARAZZO';
    if (contentType === 'ARAZZO' && data.apiFlowDefinition) {
        return inferApiIdsFromArazzo(data.apiFlowDefinition, checkboxes);
    }
    if (contentType === 'MD' && data.markdownContent) {
        return inferApiIdsFromMarkdown(data.markdownContent, checkboxes);
    }
    return [];
}

function inferApiIdsFromArazzo(specContent, checkboxes) {
    let spec = null;
    try { spec = JSON.parse(specContent); } catch { /* not JSON */ }
    if (!spec && window.jsyaml) {
        try { spec = window.jsyaml.load(specContent); } catch { /* not valid */ }
    }
    if (!spec || !Array.isArray(spec.sourceDescriptions)) return [];

    const sdNames = spec.sourceDescriptions.map(sd => (sd.name || '').toLowerCase().trim());
    const sdUrls = spec.sourceDescriptions.map(sd => (sd.url || '').toLowerCase());

    return checkboxes
        .filter(cb => {
            const name = (cb.dataset.apiName || '').toLowerCase().trim();
            const handle = (cb.dataset.apiHandle || '').toLowerCase().trim();
            return sdNames.includes(name)
                || sdNames.includes(handle)
                || sdUrls.some(url => handle && url.includes('/' + handle + '/'));
        })
        .map(cb => cb.value);
}

function inferApiIdsFromMarkdown(mdContent, checkboxes) {
    const lower = mdContent.toLowerCase();
    return checkboxes
        .filter(cb => {
            const name = (cb.dataset.apiName || '').toLowerCase().trim();
            const handle = (cb.dataset.apiHandle || '').toLowerCase().trim();
            const escape = s => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
            const matches = token => token.length >= 3 && new RegExp('\\b' + escape(token) + '\\b', 'i').test(lower);
            return matches(name) || matches(handle);
        })
        .map(cb => cb.value);
}

function openEditApiFlow(apiFlowId) {
    const data = (window.apiFlowsData || apiFlowsData || []).find(f => String(f.apiFlowId) === String(apiFlowId));
    if (!data) return;
    resetApiFlowForm();
    document.getElementById('apiFlowFormTitle').textContent = 'Edit API Flow';
    document.getElementById('editingApiFlowId').value = apiFlowId;
    const nameField = document.getElementById('apiFlowName');
    if (nameField) { nameField.value = data.name || ''; nameField.readOnly = true; nameField.classList.add('af-field-readonly'); }

    document.getElementById('apiFlowDescription').value = data.description || '';

    currentContentType = data.contentType || 'ARAZZO';
    createPathFormat = currentContentType === 'MD' ? 'markdown' : 'arazzo';

    const arazoVal = data.apiFlowDefinition || '';
    document.getElementById('apiFlowDefinition').value = arazoVal;
    if (arazoEditor) arazoEditor.setValue(arazoVal);
    updateCopyArazzoBtn(arazoVal.trim().length > 0);

    const mdVal = data.markdownContent || '';
    document.getElementById('markdownContent').value = mdVal;
    updateMarkdownWordCount();

    document.getElementById('agentPromptField').value = data.agentPrompt || '';
    setSaveButtonMode('edit', data.status);

    const visibilityRadio = document.querySelector(`input[name="apiFlowVisibility"][value="${data.visibility || 'PUBLIC'}"]`);
    if (visibilityRadio) visibilityRadio.checked = true;
    const agentVisibilityRadio = document.querySelector(`input[name="apiFlowAgentVisibility"][value="${data.agentVisibility || 'VISIBLE'}"]`);
    if (agentVisibilityRadio) agentVisibilityRadio.checked = true;
    syncAccessMatrixFromRadios();
    syncAgentPromptTab(data.agentVisibility === 'HIDDEN');

    const preSelectedIds = inferApiIdsFromContent(data);
    setPickerSelection(preSelectedIds);

    // In edit mode use "Create from Template" path so APIs + editor are visible,
    // but honour the stored content type when deciding which editor to show.
    switchWorkflowPath('create');
    switchCreateFormat(createPathFormat);

    // Disable format toggle in edit mode — format cannot be changed after creation.
    document.querySelectorAll('.af-create-format-btn').forEach(btn => {
        btn.disabled = true;
        btn.title = 'Format cannot be changed when editing';
    });

    updateSectionSummaries();
    updateStep1Preview();

    const listSection = document.getElementById('apiFlowList');
    const formSection = document.getElementById('apiFlowForm');
    listSection.style.display = 'none';
    formSection.style.display = 'block';
}

// ─────────────────────────────────────────────
// Agent Prompt modal
// ─────────────────────────────────────────────

function openPromptModal(apiFlowId) {
    const data = (window.apiFlowsData || apiFlowsData || []).find(f => String(f.apiFlowId) === String(apiFlowId));
    if (!data) return;
    document.getElementById('agentPromptFlowName').textContent = data.name;
    document.getElementById('agentPromptContent').textContent = data.agentPrompt || '';
    const copyIcon = document.getElementById('copyPromptBtn')?.querySelector('i');
    if (copyIcon) copyIcon.className = 'bi bi-copy';
    const modal = new bootstrap.Modal(document.getElementById('agentPromptModal'));
    modal.show();
}

function copyPrompt() {
    const content = document.getElementById('agentPromptContent').textContent;
    navigator.clipboard.writeText(content).then(() => {
        const btn = document.getElementById('copyPromptBtn');
        const icon = btn.querySelector('i');
        icon.className = 'bi bi-clipboard-check';
        setTimeout(() => { icon.className = 'bi bi-clipboard'; }, 2000);
    });
}

function downloadPrompt() {
    const content = document.getElementById('agentPromptContent').textContent;
    const name = document.getElementById('agentPromptFlowName').textContent || 'agent-prompt';
    const blob = new Blob([content], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = name.toLowerCase().replace(/\s+/g, '-') + '.txt';
    a.click();
    URL.revokeObjectURL(url);
}

function copyFieldPrompt() {
    const content = document.getElementById('agentPromptField').value;
    navigator.clipboard.writeText(content).then(() => {
        showAlert('Agent prompt copied to clipboard', 'success');
    });
}

function sanitizeInput(input) {
    const div = document.createElement('div');
    div.appendChild(document.createTextNode(String(input)));
    return div.innerHTML;
}

// ─────────────────────────────────────────────
// Arazzo Spec Generation (from selected APIs)
// ─────────────────────────────────────────────

function slugify(text) {
    return (text || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'flow';
}

function buildArazzoSpec(name, description, apis, orgHandle, viewName) {
    const slug = slugify(name);

    const sourceDescriptions = apis.map(api => {
        const handle = api.apiHandle || api.API_HANDLE || '';
        const path = orgHandle
            ? `/${orgHandle}/views/${viewName}/api/${handle}/docs/specification.json`
            : `/views/${viewName}/api/${handle}/docs/specification.json`;
        const url = `${window.location.origin}${path}`;
        const apiName = api.apiName || api.API_NAME || '';
        const sdName = apiName || handle || 'api-' + (apis.indexOf(api) + 1);
        return `  - name: '${sdName.replace(/'/g, "''")}'\n    url: '${url}'\n    type: openapi`;
    }).join('\n');

    return `# IMPORTANT: THIS IS A GENERATED TEMPLATE AND IS NOT COMPLETE.
# You must fill in the 'inputs', 'steps', and 'outputs' sections before this workflow can be executed.
# Read each source's OpenAPI spec to determine the correct operations, parameters, and response shapes.

arazzo: '1.0.0'
info:
  title: '${(name || 'My Flow').replace(/'/g, "''")}'
  version: 1.0.0
  description: '${(description || '').replace(/'/g, "''")}'

# Each entry below points to an OpenAPI spec. Read every spec before writing any step —
# the operations, parameters, and response shapes in those specs drive everything below.
${apis.length > 0 ? 'sourceDescriptions:\n' + sourceDescriptions : 'sourceDescriptions: []'}

workflows:
  - workflowId: ${slug}
    summary: '${(description || '').replace(/'/g, "''").slice(0, 80)}'
    description: >
      ${(description || '').replace(/'/g, "''")}

    # ── INPUTS ──────────────────────────────────────────────────────────────
    # Declare every value the caller must supply to start this workflow.
    inputs:
      type: object
      properties: {}

    # ── STEPS ───────────────────────────────────────────────────────────────
    # Read the specs, determine execution order, write one step per API operation.
    steps: []

    # ── WORKFLOW-LEVEL OUTPUTS ───────────────────────────────────────────────
    outputs: {}
`;
}

function generateArazzoSpec() {
    if (createPathFormat === 'markdown') {
        generateMarkdownTemplate();
        return;
    }

    const name = document.getElementById('apiFlowName')?.value?.trim() || '';
    const description = document.getElementById('apiFlowDescription')?.value?.trim() || '';
    const apis = getSelectedAPIs();
    const pathParts = window.location.pathname.split('/');
    const orgHandle = pathParts[1] || '';
    const viewName = pathParts[3] || 'default';

    const spec = buildArazzoSpec(name, description, apis, orgHandle, viewName);

    if (arazoEditor) {
        arazoEditor.setValue(spec);
    } else {
        const field = document.getElementById('apiFlowDefinition');
        if (field) field.value = spec;
    }

    document.getElementById('apiFlowDefinitionWrapper')?.classList.remove('d-none');
    document.getElementById('specEditorCard')?.classList.remove('d-none');
    setTimeout(() => arazoEditor?.refresh(), 50);
    updateCopyArazzoBtn(spec.trim().length > 0);
}

function generateMarkdownTemplate() {
    const name = document.getElementById('apiFlowName')?.value?.trim() || 'My Workflow';
    const description = document.getElementById('apiFlowDescription')?.value?.trim() || '';
    const apis = getSelectedAPIs();

    const apiSection = apis.length > 0
        ? apis.map(a => `- **${a.apiName}**${a.apiType ? ` (${a.apiType})` : ''}: ${a.apiDescription || 'No description'}`).join('\n')
        : '_No APIs selected yet — add them above._';

    const template = `# ${name}

${description || '_Add a description for this workflow._'}

## APIs Used

${apiSection}

## Trigger

_Describe what initiates this workflow — a user action, a scheduled event, a webhook, etc._

## Steps

1. **Step name** — Describe the first API call: what is sent, what is returned, and why it comes first.
2. **Step name** — Describe the next call. Reference outputs from the previous step where relevant.
3. **Step name** — Continue for each distinct operation in the workflow.

## Success Outcome

_Describe what a successful execution produces for the caller._

## Error Handling

| Scenario | Behaviour |
|---|---|
| _e.g. Authentication fails_ | _Return 401, abort workflow_ |
| _e.g. Resource not found_ | _Return 404 with error detail_ |
`;

    const mdField = document.getElementById('markdownContent');
    if (mdField) {
        mdField.value = template;
        updateMarkdownWordCount();
    }

    document.getElementById('markdownContentWrapper')?.classList.remove('d-none');
    document.getElementById('specEditorCard')?.classList.remove('d-none');
}

function copyArazzoSpec() {
    const content = arazoEditor ? arazoEditor.getValue() : (document.getElementById('apiFlowDefinition')?.value || '');
    navigator.clipboard.writeText(content).then(() => showAlert('Arazzo spec copied to clipboard', 'success'));
}

// ─────────────────────────────────────────────
// CodeMirror Editor
// ─────────────────────────────────────────────

function initCodeMirrorEditor() {
    const host = document.getElementById('arazoEditorHost');
    if (!host || !window.CodeMirror) return;

    arazoEditor = CodeMirror(host, {
        value: document.getElementById('apiFlowDefinition')?.value || '',
        mode: 'yaml',
        lineNumbers: true,
        tabSize: 2,
        indentWithTabs: false,
        lineWrapping: true,
        extraKeys: { 'Tab': cm => cm.execCommand('insertSoftTab') }
    });
    arazoEditor.setSize(null, '420px');

    let sdValidationTimer = null;
    arazoEditor.on('change', () => {
        clearTimeout(sdValidationTimer);
        const content = arazoEditor.getValue();
        updateArazzoEditorUI(content);
        updateWorkflowMdPreview();
        sdValidationTimer = setTimeout(() => {
            if (content.trim()) validateAndRenderSourceDescriptions(content);
        }, 800);
    });
}

function updateArazzoEditorUI(content) {
    const hasContent = content.trim().length > 0;
    const lineCount = content ? content.split('\n').length : 0;

    // Line count in bar
    const lineCountEl = document.getElementById('arazoLineCount');
    if (lineCountEl) lineCountEl.textContent = lineCount === 1 && !content.trim() ? '0 lines' : `${lineCount} line${lineCount === 1 ? '' : 's'}`;

    // Empty state overlay
    const emptyState = document.getElementById('arazoEmptyState');
    if (emptyState) {
        emptyState.style.display = hasContent ? 'none' : '';
        emptyState.setAttribute('aria-hidden', String(hasContent));
    }

    // Copy button
    const btn = document.getElementById('copySpecBtn');
    if (btn) {
        btn.disabled = !hasContent;
        btn.title = hasContent ? 'Copy Arazzo spec from editor' : 'Generate a spec first';
    }

    // Footer status text
    const status = document.getElementById('editorStatusText');
    if (status) status.textContent = hasContent ? `${lineCount} line${lineCount === 1 ? '' : 's'}` : 'Editor is empty';
}

function updateCopyArazzoBtn(hasContent) {
    const content = hasContent
        ? (arazoEditor ? arazoEditor.getValue() : (document.getElementById('apiFlowDefinition')?.value || ''))
        : '';
    updateArazzoEditorUI(content);
}

function updateEditorFooter(format) {
    const labelEl = document.getElementById('copySpecBtnLabel');
    const copyBtn = document.getElementById('copySpecBtn');
    if (labelEl) labelEl.textContent = format === 'markdown' ? 'Copy from editor' : 'Copy Arazzo spec from editor';
    if (copyBtn) copyBtn.title = format === 'markdown' ? 'Copy to clipboard' : 'Generate a spec first';
    if (format === 'markdown') {
        const mdContent = document.getElementById('markdownContent')?.value || '';
        if (copyBtn) {
            copyBtn.disabled = !mdContent.trim();
            copyBtn.title = mdContent.trim() ? 'Copy to clipboard' : 'Add content first';
        }
        const status = document.getElementById('editorStatusText');
        if (status) status.textContent = mdContent.trim() ? `${mdContent.split('\n').length} lines` : 'Editor is empty';
    } else {
        const content = arazoEditor ? arazoEditor.getValue() : (document.getElementById('apiFlowDefinition')?.value || '');
        updateArazzoEditorUI(content);
    }
}

// ─────────────────────────────────────────────
// Section Collapse
// ─────────────────────────────────────────────

function initSectionCollapse() {
    document.querySelectorAll('[data-af-collapse]').forEach(btn => {
        btn.addEventListener('click', () => {
            const section = btn.closest('.af-section');
            const body = section?.querySelector('.af-section-body');
            const summary = section?.querySelector('.af-section-summary');
            const icon = btn.querySelector('i');
            if (!body) return;
            const isCollapsed = body.classList.contains('af-section-body--collapsed');
            body.classList.toggle('af-section-body--collapsed', !isCollapsed);
            if (summary) summary.classList.toggle('d-none', isCollapsed);
            if (icon) icon.className = isCollapsed ? 'bi bi-chevron-up' : 'bi bi-chevron-down';
            if (isCollapsed && arazoEditor) arazoEditor.refresh();
        });
    });
}

function expandAllSections() {
    document.querySelectorAll('.af-section-body--collapsed').forEach(body => {
        body.classList.remove('af-section-body--collapsed');
    });
    document.querySelectorAll('.af-section-summary').forEach(s => s.classList.add('d-none'));
    document.querySelectorAll('[data-af-collapse] i').forEach(i => { i.className = 'bi bi-chevron-up'; });
}

// ─────────────────────────────────────────────
// Section Summaries
// ─────────────────────────────────────────────

function updateSectionSummaries() {
    const name = document.getElementById('apiFlowName')?.value?.trim() || '';
    const vis = document.querySelector('input[name="apiFlowVisibility"]:checked')?.value || 'PUBLIC';
    const agentVis = document.querySelector('input[name="apiFlowAgentVisibility"]:checked')?.value || 'VISIBLE';
    const s1 = document.getElementById('af-summary-1');
    if (s1) {
        s1.textContent = name
            ? `${name} · ${vis.charAt(0) + vis.slice(1).toLowerCase()} · Agent ${agentVis.charAt(0) + agentVis.slice(1).toLowerCase()}`
            : 'Not configured';
    }

    const s2 = document.getElementById('af-summary-2');
    if (s2) {
        const hasArazzo = arazoEditor
            ? arazoEditor.getValue().trim().length > 0
            : (document.getElementById('apiFlowDefinition')?.value?.trim().length > 0);
        const hasMd = (document.getElementById('markdownContent')?.value?.trim().length > 0);
        const hasContent = currentContentType === 'MD' ? hasMd : hasArazzo;
        const fmtLabel = currentContentType === 'MD' ? 'Markdown' : 'Arazzo';
        const pathLabel = currentWorkflowPath === 'upload' ? 'Upload' : 'Template';
        s2.textContent = hasContent ? `${pathLabel} · ${fmtLabel} loaded` : pathLabel;
    }

    const agentPrompt = document.getElementById('agentPromptField')?.value?.trim();
    const s3 = document.getElementById('af-summary-3');
    if (s3) s3.textContent = agentPrompt ? 'Prompt ready' : 'Not generated';
}

// ─────────────────────────────────────────────
// Selected API Chips
// ─────────────────────────────────────────────

// ─────────────────────────────────────────────
// Markdown Helpers
// ─────────────────────────────────────────────

function copyMarkdown() {
    const content = document.getElementById('markdownContent')?.value || '';
    navigator.clipboard.writeText(content).then(() => showAlert('Copied to clipboard', 'success'));
}

function updateMarkdownWordCount() {
    const field = document.getElementById('markdownContent');
    const counter = document.getElementById('mdWordCount');
    if (!field) return;
    const words = field.value.trim().split(/\s+/).filter(Boolean).length;
    if (counter) counter.textContent = words > 0 ? `${words.toLocaleString()} words` : '';
    // Keep footer status in sync when in markdown mode
    if (createPathFormat === 'markdown') {
        const lines = field.value ? field.value.split('\n').length : 0;
        const status = document.getElementById('editorStatusText');
        if (status) status.textContent = field.value.trim() ? `${lines} line${lines === 1 ? '' : 's'}` : 'Editor is empty';
    }
}

function updateApiChips() {
    const chips = document.getElementById('apiSelectedChips');
    if (!chips) return;
    const selected = [...document.querySelectorAll('.api-flow-api-checkbox:checked')];
    if (selected.length === 0) {
        chips.innerHTML = '';
        chips.style.display = 'none';
    } else {
        chips.innerHTML = selected.map(cb =>
            `<span class="af-api-chip">${sanitizeInput(cb.dataset.apiName)}</span>`
        ).join('');
        chips.style.display = '';
    }
}

// ─────────────────────────────────────────────
// Wizard Navigation
// ─────────────────────────────────────────────

function initWizard() {
    document.getElementById('afContinueBtn')?.addEventListener('click', () => {
        if (validateWizardStep(currentStep)) goToStep(currentStep + 1);
    });
    // Clicking or keyboard-activating a complete stepper step navigates back
    document.querySelectorAll('.af-stepper-step').forEach(el => {
        el.addEventListener('click', () => {
            const s = parseInt(el.dataset.step);
            if (s < currentStep) goToStep(s);
        });
        el.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                const s = parseInt(el.dataset.step);
                if (s < currentStep) goToStep(s);
            }
        });
    });

    updateWizardUI();
    updateStep1Preview();
    updateStep3Readiness();

    // Live preview updates + clear validation errors on input
    document.getElementById('apiFlowName')?.addEventListener('input', (e) => {
        if (e.target.value.trim()) e.target.classList.remove('is-invalid');
        updateStep1Preview();
        updateStep3Readiness();
        updateWorkflowMdPreview();
    });
    document.getElementById('apiFlowDescription')?.addEventListener('input', (e) => {
        if (e.target.value.trim()) e.target.classList.remove('is-invalid');
        updateStep1Preview();
        updateStep3Readiness();
        updateWorkflowMdPreview();
    });
    document.getElementById('agentPromptField')?.addEventListener('input', updateStep3Readiness);

}

function goToStep(n) {
    if (n < 1 || n > 3) return;
    currentStep = n;
    updateWizardUI();
    if (n === 2) setTimeout(() => arazoEditor?.refresh(), 80);
    if (n === 3) updateStep3Readiness();
}

function validateWizardStep(step) {
    if (step === 1) {
        const name = document.getElementById('apiFlowName');
        const desc = document.getElementById('apiFlowDescription');
        let valid = true;
        if (!name?.value.trim()) { name?.classList.add('is-invalid'); valid = false; } else name?.classList.remove('is-invalid');
        if (!desc?.value.trim()) { desc?.classList.add('is-invalid'); valid = false; } else desc?.classList.remove('is-invalid');
        return valid;
    }
    if (step === 2) {
        const isMarkdown = currentContentType === 'MD';
        const hasContent = isMarkdown
            ? document.getElementById('markdownContent')?.value?.trim().length > 0
            : arazoEditor ? arazoEditor.getValue().trim().length > 0 : document.getElementById('apiFlowDefinition')?.value?.trim().length > 0;
        if (!hasContent) {
            showAlert('Add an API workflow spec before continuing', 'warning');
            return false;
        }
        return true;
    }
    return true;
}

function updateWizardUI() {
    // Single-column layout for step 2
    document.querySelector('.af-wizard-body')?.classList.toggle('af-wizard-body--single-col', currentStep === 2);
    // Step panels
    for (let i = 1; i <= 3; i++) {
        document.getElementById(`afStep${i}`)?.classList.toggle('d-none', i !== currentStep);
        document.getElementById(`afRight${i}`)?.classList.toggle('d-none', i !== currentStep);
    }
    // Stepper dots + ARIA state
    document.querySelectorAll('.af-stepper-step').forEach(el => {
        const s = parseInt(el.dataset.step);
        el.classList.toggle('is-active', s === currentStep);
        el.classList.toggle('is-complete', s < currentStep);
        if (s > currentStep) {
            el.classList.remove('is-active', 'is-complete');
        }
        const isComplete = s < currentStep;
        const isCurrent = s === currentStep;
        el.setAttribute('aria-current', isCurrent ? 'step' : 'false');
        el.setAttribute('aria-disabled', isComplete ? 'false' : 'true');
        el.setAttribute('tabindex', isComplete ? '0' : '-1');
        const dot = el.querySelector('.af-stepper-dot');
        if (dot) dot.innerHTML = s < currentStep ? '<i class="bi bi-check2"></i>' : String(s);
    });
    // Stepper lines
    document.querySelectorAll('.af-stepper-line').forEach((line, idx) => {
        line.classList.toggle('is-complete', idx + 1 < currentStep);
    });
    // Footer
    const label = document.getElementById('afFooterStepLabel');
    if (label) label.textContent = `Step ${currentStep} of 3`;
    const continueBtn = document.getElementById('afContinueBtn');
    if (continueBtn) continueBtn.classList.toggle('d-none', currentStep === 3);
    const saveGroup = document.getElementById('saveApiFlowGroup');
    if (saveGroup) saveGroup.classList.toggle('d-none', currentStep !== 3);
}

// ─────────────────────────────────────────────
// Step 1 Right Pane — Live Preview
// ─────────────────────────────────────────────

function updateStep1Preview() {
    const name = document.getElementById('apiFlowName')?.value?.trim() || '';
    const desc = document.getElementById('apiFlowDescription')?.value?.trim() || '';
    const vis = document.querySelector('input[name="apiFlowVisibility"]:checked')?.value || 'PUBLIC';

    // Preview card
    const nameEl = document.getElementById('afPreviewName');
    const emptyEl = document.getElementById('afPreviewNameEmpty');
    if (nameEl) nameEl.textContent = name;
    if (emptyEl) emptyEl.classList.toggle('d-none', !!name);

    const descEl = document.getElementById('afPreviewDesc');
    if (descEl) descEl.textContent = desc;
    const badge = document.getElementById('afPreviewBadge');
    if (badge) {
        badge.textContent = vis.toLowerCase();
        badge.className = `af-preview-badge${vis === 'PUBLIC' ? ' af-preview-badge--public' : ''}`;
    }

    // Inline field status icons
    if (name.length === 0) {
        updateFieldStatus('afNameStatus', 'hidden', '');
    } else if (name.length < 5) {
        updateFieldStatus('afNameStatus', 'warn', 'Name is too short — aim for at least 5 characters');
    } else {
        updateFieldStatus('afNameStatus', 'ok', '');
    }

    if (desc.length === 0) {
        updateFieldStatus('afDescStatus', 'hidden', '');
    } else if (desc.length < 20) {
        updateFieldStatus('afDescStatus', 'warn', 'Description is too short — aim for at least 20 characters');
    } else {
        updateFieldStatus('afDescStatus', 'ok', '');
    }

    // Sync access matrix visual state
    syncAccessMatrixFromRadios();
}

function updateFieldStatus(statusId, state, tooltipMsg) {
    const el = document.getElementById(statusId);
    if (!el) return;
    const icon = el.querySelector('i');
    if (state === 'hidden') {
        el.classList.add('d-none');
        bootstrap.Tooltip.getInstance(el)?.dispose();
        return;
    }
    el.classList.remove('d-none');
    if (icon) {
        icon.className = 'bi af-fsi';
        if (state === 'ok') icon.classList.add('bi-check-circle-fill', 'af-fsi-ok');
        else icon.classList.add('bi-exclamation-circle-fill', 'af-fsi-warn');
    }
    if (tooltipMsg) {
        el.setAttribute('data-bs-title', tooltipMsg);
        const existing = bootstrap.Tooltip.getInstance(el);
        if (existing) existing.setContent({ '.tooltip-inner': tooltipMsg });
        else new bootstrap.Tooltip(el);
    } else {
        bootstrap.Tooltip.getInstance(el)?.dispose();
    }
}

function setChecklistItem(id, state) {
    const el = document.getElementById(id);
    if (!el) return;
    el.classList.remove('is-ok', 'is-warn', 'is-info');
    if (state) el.classList.add(`is-${state}`);
}

// ─────────────────────────────────────────────
// Step 3 Right Pane — Readiness
// ─────────────────────────────────────────────

function updateStep3Readiness() {
    const name = document.getElementById('apiFlowName')?.value?.trim() || '';
    const desc = document.getElementById('apiFlowDescription')?.value?.trim() || '';
    const hasArazzo = arazoEditor ? arazoEditor.getValue().trim().length > 0 : (document.getElementById('apiFlowDefinition')?.value?.trim().length > 0);
    const hasMd = document.getElementById('markdownContent')?.value?.trim().length > 0;
    const hasSpec = hasArazzo || hasMd;
    const hasPrompt = document.getElementById('agentPromptField')?.value?.trim().length > 0;

    setChecklistItem('afReady1', (name && desc) ? 'ok' : 'warn');
    setChecklistItem('afReady2', hasSpec ? 'ok' : 'warn');
    setChecklistItem('afReady3', hasPrompt ? 'ok' : 'warn');

    // Sync step 3 preview name/desc (kept for any external usage)
    const n3 = document.getElementById('afPreviewName3');
    if (n3) n3.textContent = name;
    const d3 = document.getElementById('afPreviewDesc3');
    if (d3) d3.textContent = desc;

    updateWorkflowMdPreview();
}

// ─────────────────────────────────────────────
// Access Visibility Cards
// ─────────────────────────────────────────────

function initAccessMatrix() {
    const bind = (id, type, value) => {
        const el = document.getElementById(id);
        if (!el) return;
        const activate = () => setAccessValue(type, value);
        el.addEventListener('click', activate);
        el.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ' ' || e.key === 'Spacebar') {
                if (e.key !== 'Enter') e.preventDefault();
                activate();
            }
        });
    };
    bind('portalPublicCard', 'visibility', 'PUBLIC');
    bind('portalPrivateCard', 'visibility', 'PRIVATE');
    bind('agentVisibleCard', 'agent', 'VISIBLE');
    bind('agentHiddenCard', 'agent', 'HIDDEN');
    syncAccessMatrixFromRadios();
}

function setAccessValue(type, value) {
    if (type === 'visibility') {
        const radio = document.querySelector(`input[name="apiFlowVisibility"][value="${value}"]`);
        if (radio) { radio.checked = true; radio.dispatchEvent(new Event('change', { bubbles: true })); }
    } else {
        const radio = document.querySelector(`input[name="apiFlowAgentVisibility"][value="${value}"]`);
        if (radio) { radio.checked = true; radio.dispatchEvent(new Event('change', { bubbles: true })); }
    }
    syncAccessMatrixFromRadios();
    updateStep1Preview();
}

function syncAccessMatrixFromRadios() {
    const vis = document.querySelector('input[name="apiFlowVisibility"]:checked')?.value || 'PUBLIC';
    const agentVis = document.querySelector('input[name="apiFlowAgentVisibility"]:checked')?.value || 'VISIBLE';

    const portalPublic = document.getElementById('portalPublicCard');
    const portalPrivate = document.getElementById('portalPrivateCard');
    const agentVisible = document.getElementById('agentVisibleCard');
    const agentHidden = document.getElementById('agentHiddenCard');

    if (portalPublic) {
        portalPublic.classList.toggle('af-visibility-card--active', vis === 'PUBLIC');
        portalPublic.setAttribute('aria-pressed', String(vis === 'PUBLIC'));
    }
    if (portalPrivate) {
        portalPrivate.classList.toggle('af-visibility-card--active', vis === 'PRIVATE');
        portalPrivate.setAttribute('aria-pressed', String(vis === 'PRIVATE'));
    }
    if (agentVisible) {
        agentVisible.classList.toggle('af-visibility-card--active', agentVis === 'VISIBLE');
        agentVisible.setAttribute('aria-pressed', String(agentVis === 'VISIBLE'));
    }
    if (agentHidden) {
        agentHidden.classList.toggle('af-visibility-card--active', agentVis === 'HIDDEN');
        agentHidden.setAttribute('aria-pressed', String(agentVis === 'HIDDEN'));
    }
}
// LLM Instructions Tab
// ─────────────────────────────────────────────

let llmsOrgID = '';
let llmsViewName = '';
let llmsBaseUrl = '';
let llmsPreviewDebounce = null;

function initLlmsConfig() {
    const dataEl = document.getElementById('llmsConfigData');
    const ctxEl = document.getElementById('llmsConfigContext');
    if (!dataEl || !ctxEl) return;

    let config = {};
    let ctx = {};
    try { config = JSON.parse(dataEl.textContent) || {}; } catch (e) { /* ignore */ }
    try { ctx = JSON.parse(ctxEl.textContent) || {}; } catch (e) { /* ignore */ }

    llmsOrgID = ctx.orgID || '';
    llmsViewName = ctx.viewName || '';
    llmsBaseUrl = ctx.baseUrl || '';
    csrfToken = ctx.csrfToken || csrfToken;

    const toggle = document.getElementById('aiEnabledToggle');
    const nameEl = document.getElementById('llmsPortalName');
    const descEl = document.getElementById('llmsPortalDescription');
    const formArea = document.getElementById('llmsConfigFormArea');

    if (toggle) {
        toggle.checked = config.aiEnabled !== false;
        toggle.addEventListener('change', () => {
            if (formArea) formArea.style.opacity = toggle.checked ? '1' : '0.4';
            if (formArea) formArea.style.pointerEvents = toggle.checked ? '' : 'none';
            scheduleLlmsPreview();
        });
        if (formArea) {
            formArea.style.opacity = toggle.checked ? '1' : '0.4';
            formArea.style.pointerEvents = toggle.checked ? '' : 'none';
        }
    }

    if (nameEl) nameEl.value = config.portalName || '';
    if (descEl) descEl.value = config.portalDescription || '';

    [nameEl, descEl].forEach(el => {
        if (el) el.addEventListener('input', scheduleLlmsPreview);
    });

    const saveBtn = document.getElementById('saveLlmsConfigBtn');
    if (saveBtn) saveBtn.addEventListener('click', saveLlmsConfig);

    document.querySelectorAll('[data-bs-toggle="tooltip"]').forEach(el => {
        new bootstrap.Tooltip(el);
    });

    scheduleLlmsPreview();
}

function scheduleLlmsPreview() {
    clearTimeout(llmsPreviewDebounce);
    llmsPreviewDebounce = setTimeout(fetchLlmsPreview, 600);
}

async function fetchLlmsPreview() {
    const toggle = document.getElementById('aiEnabledToggle');
    const previewEl = document.getElementById('llmsPreviewContent');
    const refreshingEl = document.getElementById('llmsPreviewRefreshing');
    if (!previewEl) return;

    if (toggle && !toggle.checked) {
        previewEl.textContent = '(llms.txt is disabled — AI agents will receive 404)';
        return;
    }

    if (refreshingEl) refreshingEl.style.display = '';
    try {
        const response = await fetch(`${llmsBaseUrl}/llms.txt/preview`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
            credentials: 'same-origin',
            body: JSON.stringify({
                portalName: document.getElementById('llmsPortalName')?.value || '',
                portalDescription: document.getElementById('llmsPortalDescription')?.value || '',
            })
        });
        const text = await response.text();
        previewEl.textContent = text;
    } catch (e) {
        previewEl.textContent = 'Preview unavailable.';
    } finally {
        if (refreshingEl) refreshingEl.style.display = 'none';
    }
}

async function saveLlmsConfig() {
    const saveBtn = document.getElementById('saveLlmsConfigBtn');
    if (saveBtn) {
        saveBtn.disabled = true;
        saveBtn.innerHTML = '<span class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span> Publishing…';
    }
    try {
        const response = await fetch(`${llmsBaseUrl}/llms-config`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
            credentials: 'same-origin',
            body: JSON.stringify({
                aiEnabled: document.getElementById('aiEnabledToggle')?.checked !== false,
                portalName: document.getElementById('llmsPortalName')?.value || '',
                portalDescription: document.getElementById('llmsPortalDescription')?.value || '',
            })
        });
        if (response.ok) {
            showAlert('LLM Instructions saved successfully', 'success');
        } else {
            const err = await response.json().catch(() => ({ message: 'Save failed' }));
            showAlert(err.message || 'Save failed', 'error');
        }
    } catch (e) {
        showAlert(e.message || 'Network error', 'error');
    } finally {
        if (saveBtn) {
            saveBtn.disabled = false;
            saveBtn.innerHTML = '<i class="bi bi-floppy me-1"></i> Publish';
        }

    }
}

document.addEventListener('DOMContentLoaded', function () {
    initLlmsConfig();

    const llmsBtn      = document.getElementById('llms-tab-btn');
    const workflowsBtn = document.getElementById('workflows-tab-btn');

    function syncTabFromHash() {
        if (window.location.hash === '#apiflows' && workflowsBtn) activateTab(workflowsBtn);
        else if (llmsBtn) activateTab(llmsBtn);
    }

    // On fresh page load: activate the correct tab based on URL hash.
    syncTabFromHash();

    // On same-page hash navigation (e.g. clicking the Settings sidebar link
    // while already on the configure page): re-sync without a full reload.
    window.addEventListener('hashchange', syncTabFromHash);
});
