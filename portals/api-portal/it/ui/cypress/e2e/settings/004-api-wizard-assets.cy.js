// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

// The API wizard's last step, Assets: the documents view changes documents only (sent with
// the API save); "Import content from ZIP" opens the import view, which replaces all content
// with one ZIP (PUT /assets), asking first when the API already exists.

const { contentPackage, invalidContentPackage } = require('../../support/contentPackages');
const { createZip } = require('../../support/zip');

describe('Settings — API wizard Assets step', () => {
    const uid = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const handles = [];
    const settingsUrl = () => `${Cypress.env('BASE_PATH')}/${Cypress.env('ORG_HANDLE')}/settings`;
    const SPEC = JSON.stringify({ openapi: '3.0.3', info: { title: 'Wizard', version: '1.0.0' }, paths: {} });
    const JPEG = { contents: Cypress.Buffer.from('ffd8ffe000104a46494600010100000100010000ffd9', 'hex'), fileName: 'logo.jpg', mimeType: 'image/jpeg' };

    const asset = (handle, type, fileName) => cy.apiRequest('GET',
        `/api/v0.9/apis/${handle}/assets?type=${type}&fileName=${encodeURIComponent(fileName)}`,
        { failOnStatusCode: false });
    const docFile = (name, text = `# ${name}`) => ({ contents: Cypress.Buffer.from(text), fileName: name, mimeType: 'text/markdown' });

    const openSettings = () => {
        cy.login();
        cy.visit(settingsUrl());
        cy.get('.cfg-nav-item[data-panel="cfg-apis"]').click();
    };
    const toAssets = () => {
        cy.get('#cfg-wizard-next').click();
        cy.get('#cfg-wizard-next').click();
        cy.get('#cfg-wstep-2').scrollIntoView().should('be.visible');
    };
    const openEdit = (handle) => {
        openSettings();
        cy.get(`.cfg-edit-trigger[data-id="${handle}"]`).first().click({ force: true });
        toAssets();
    };
    const openCreate = (handle) => {
        openSettings();
        cy.get('#cfg-add-api-btn').click();
        cy.get('#wz-name').type(`Wizard ${handle}`);
        cy.get('#wz-version').type('v1.0');
        cy.get('#wz-handle').clear().type(handle);
        cy.get('#wz-type').select('RestApi');
        cy.get('#wz-prod').type('https://backend.example.invalid/pets');
        cy.get('#cfg-wizard-next').click();
        cy.get('#wz-spec-input').selectFile({ contents: Cypress.Buffer.from(SPEC), fileName: 'definition.json' }, { force: true });
        cy.get('#cfg-wizard-next').click();
        cy.get('#cfg-wstep-2').scrollIntoView().should('be.visible');
        handles.push(handle);
    };
    const toImport = () => cy.get('#wz-assets-to-import').click();
    const save = () => cy.get('#cfg-wizard-next').click();
    // A successful save reloads the settings page, which hides the wizard again.
    const saved = () => cy.get('#cfg-apis-wizard', { timeout: 20000 }).should('not.be.visible');

    after(() => {
        cy.login();
        handles.forEach((handle) => cy.deleteApi(handle));
    });

    it('shows three steps and the create text, and saves uploaded Markdown docs with the API', () => {
        const handle = `it-wizard-create-docs-${uid}`;
        openCreate(handle);

        cy.get('#cfg-apis-wizard .cfg-step-label').then(($labels) => {
            expect([...$labels].map((el) => el.textContent.trim())).to.deep.eq(['Details', 'Spec', 'Assets']);
        });
        cy.get('#cfg-wizard-next').should('have.text', 'Create API');
        cy.get('#wz-assets-desc-docs').should('be.visible')
            .and('have.text', 'Add documents for this API. You can change them later by editing the API.');
        cy.get('#wz-existing-docs-section').should('not.be.visible');
        cy.get('#wz-assets-to-import').should('be.visible').and('contain', 'Import content from ZIP');

        cy.get('#wz-docs-input').should('have.attr', 'accept', '.md');
        cy.get('#wz-docs-zone').should('contain', 'Markdown (.md) files');
        cy.get('#wz-docs-input').selectFile([docFile('A.md'), docFile('diagram.txt', 'not markdown')], { force: true });
        cy.get('#alertToast').should('have.class', 'error')
            .find('.alert-toast-message').should('have.text', 'Only Markdown (.md) documents are supported: diagram.txt');
        cy.get('#wz-docs-list .cfg-docs-item').should('have.length', 1).and('contain', 'A.md').and('contain', 'New');
        cy.get('#wz-docs-summary').should('have.text', 'When you save: 1 document will be added.');
        save();
        saved();

        asset(handle, 'DOC_Other', 'A.md').its('status').should('eq', 200);
        asset(handle, 'DOC_Other', 'diagram.txt').its('status').should('eq', 404);
    });

    it('clears the view left when switching between documents and import', () => {
        const handle = `it-wizard-switch-${uid}`;
        cy.login();
        cy.seedApi({ id: handle, name: `Wizard ${handle}`, docs: [{ name: 'A.md', content: '# A' }] });
        handles.push(handle);

        openEdit(handle);
        cy.get('#wz-assets-desc-docs').should('not.be.visible');
        cy.get('#wz-docs-input').selectFile(docFile('B.md'), { force: true });
        cy.contains('#wz-existing-docs-list .cfg-docs-item', 'A.md').find('.cfg-docs-remove').click();
        cy.get('#wz-docs-summary').should('have.text', 'When you save: 1 document will be added and 1 removed.');

        toImport();
        cy.get('#wz-assets-panel-docs').should('not.be.visible');
        cy.get('#wz-content-input').selectFile(contentPackage(), { force: true });
        cy.get('#wz-content-chip').should('have.class', 'visible').and('contain', 'Applied when you save');

        cy.get('#wz-assets-to-docs').should('contain', 'Back to uploading documents').click();
        cy.get('#wz-docs-list .cfg-docs-item').should('not.exist');
        cy.contains('#wz-existing-docs-list .cfg-docs-item', 'A.md').should('not.contain', 'Will be removed');
        cy.get('#wz-docs-summary').should('have.text', '');

        toImport();
        cy.get('#wz-content-chip').should('not.have.class', 'visible');
    });

    it('replaces a same-name doc and removes another with Undo', () => {
        const handle = `it-wizard-edit-docs-${uid}`;
        cy.login();
        cy.seedApi({ id: handle, name: `Wizard ${handle}`, docs: [{ name: 'A.md', content: '# Old A' }, { name: 'B.md', content: '# B' }] });
        handles.push(handle);

        openEdit(handle);
        cy.get('#wz-existing-docs-section').should('be.visible').and('contain', 'On this API');

        cy.get('#wz-docs-input').selectFile(docFile('A.md', '# New A'), { force: true });
        cy.contains('#wz-docs-list .cfg-docs-item', 'A.md').should('contain', 'Replaces existing');

        cy.contains('#wz-existing-docs-list .cfg-docs-item', 'B.md').find('.cfg-docs-remove').click();
        cy.contains('#wz-existing-docs-list .cfg-docs-item', 'B.md').should('contain', 'Will be removed').find('.cfg-docs-undo').click();
        cy.contains('#wz-existing-docs-list .cfg-docs-item', 'B.md').should('not.contain', 'Will be removed').find('.cfg-docs-remove').click();
        cy.get('#wz-docs-summary').should('have.text', 'When you save: 1 document will be replaced and 1 removed.');
        cy.get('#cfg-wizard-next').should('have.text', 'Save changes');
        save();
        saved();

        asset(handle, 'DOC_Other', 'A.md').its('body').should('contain', 'New A');
        asset(handle, 'DOC_Other', 'B.md').its('status').should('eq', 404);
    });

    it('imports a package on create without asking', () => {
        const handle = `it-wizard-create-import-${uid}`;
        openCreate(handle);

        toImport();
        cy.get('#wz-assets-panel-import').should('contain', 'Import a content package').and('contain', 'Expected ZIP layout');
        cy.get('#wz-assets-desc-import').should('not.be.visible');
        cy.get('#wz-content-input').selectFile(contentPackage(), { force: true });
        save();
        cy.get('#cfg-replace-content-modal').should('not.be.visible');
        saved();

        asset(handle, 'MARKETING', 'api-content.md').its('status').should('eq', 200);
        asset(handle, 'IMAGE', 'api-icon.png').its('status').should('eq', 200);
        asset(handle, 'DOC_Other', 'Guide.md').its('status').should('eq', 200);
        asset(handle, 'DOC_HowTo', 'Setup.md').its('status').should('eq', 200);
    });

    it('asks before an import replaces existing content; Cancel changes nothing, Replace content replaces everything', () => {
        const handle = `it-wizard-edit-import-${uid}`;
        cy.login();
        cy.seedApi({ id: handle, name: `Wizard ${handle}`, docs: [{ name: 'Old.md', content: '# Old' }] });
        handles.push(handle);

        openEdit(handle);
        toImport();
        cy.get('#wz-assets-desc-import').scrollIntoView().should('be.visible')
            .and('have.text', 'Saving removes the current documents, landing page and thumbnail, and uses this ZIP’s content instead.');
        cy.get('#wz-content-input').selectFile(contentPackage(), { force: true });

        save();
        cy.get('#cfg-replace-content-modal').should('be.visible').and('contain', 'Replace all content?')
            .and('contain', `This replaces all documents, links, the landing page, its images and the thumbnail of Wizard ${handle} with the content of content.zip. Continue?`);
        cy.focused().should('have.id', 'cfg-replace-content-cancel').trigger('keydown', { key: 'Tab' });
        cy.focused().should('have.id', 'cfg-replace-content-confirm').trigger('keydown', { key: 'Tab', shiftKey: true });
        cy.focused().should('have.id', 'cfg-replace-content-cancel');
        cy.get('#cfg-replace-content-cancel').click();
        cy.get('#cfg-replace-content-modal').should('not.be.visible');
        cy.focused().should('have.id', 'cfg-wizard-next');
        asset(handle, 'DOC_Other', 'Old.md').its('status').should('eq', 200);
        asset(handle, 'MARKETING', 'api-content.md').its('status').should('eq', 404);

        save();
        cy.get('#cfg-replace-content-confirm').should('have.text', 'Replace content').click();
        saved();
        asset(handle, 'DOC_Other', 'Old.md').its('status').should('eq', 404);
        asset(handle, 'DOC_Other', 'Guide.md').its('status').should('eq', 200);
        asset(handle, 'IMAGE', 'api-icon.png').its('status').should('eq', 200);

        // A later doc-only edit keeps the imported landing page and thumbnail.
        openEdit(handle);
        cy.get('#wz-docs-input').selectFile(docFile('C.md'), { force: true });
        save();
        saved();
        asset(handle, 'DOC_Other', 'C.md').its('status').should('eq', 200);
        asset(handle, 'MARKETING', 'api-content.md').its('status').should('eq', 200);
        asset(handle, 'IMAGE', 'api-icon.png').its('status').should('eq', 200);
    });

    it('applies a thumbnail change first, then the ZIP, whose thumbnail wins', () => {
        const handle = `it-wizard-thumb-zip-${uid}`;
        cy.login();
        cy.seedApi({ id: handle, name: `Wizard ${handle}` });
        handles.push(handle);

        openSettings();
        cy.get(`.cfg-edit-trigger[data-id="${handle}"]`).first().click({ force: true });
        cy.get('#wz-thumb-input').selectFile(JPEG, { force: true });
        toAssets();
        toImport();
        cy.get('#wz-content-input').selectFile(contentPackage(), { force: true });
        save();
        cy.get('#cfg-replace-content-confirm').click();
        saved();

        asset(handle, 'IMAGE', 'api-icon.png').its('status').should('eq', 200);
        asset(handle, 'IMAGE', 'api-icon.jpg').its('status').should('eq', 404);
    });

    it('keeps the API and continues as Edit when the package is rejected on create', () => {
        const handle = `it-wizard-create-bad-${uid}`;
        openCreate(handle);

        toImport();
        cy.get('#wz-content-input').selectFile(invalidContentPackage(), { force: true });
        save();

        cy.get('#wz-content-error').scrollIntoView().should('be.visible')
            .and('have.text', 'doc/: unknown top-level entry\nweb/style.css: web/ holds only api-content.md, api-content.hbs and images');
        cy.get('#wz-content-chip').should('contain', 'content.zip').and('contain', 'Not uploaded');
        cy.get('#wz-content-choose').scrollIntoView().should('be.visible').and('have.text', 'Choose another file');
        cy.get('#wz-content-remove').should('not.be.visible');
        cy.get('#cfg-wstep-2').scrollIntoView().should('be.visible');
        cy.get('#cfg-wizard-title').should('have.text', 'Edit API');
        cy.get('#cfg-wizard-next').should('have.text', 'Save changes');
        cy.apiRequest('GET', `/api/v0.9/apis/${handle}`, { failOnStatusCode: false }).its('status').should('eq', 200);

        cy.get('#wz-content-input').selectFile(contentPackage(), { force: true });
        cy.get('#wz-content-chip').should('contain', 'Applied when you save').and('not.contain', 'Not uploaded');
        cy.get('#wz-content-error').should('not.be.visible');
        save();
        saved();
        asset(handle, 'MARKETING', 'api-content.md').its('status').should('eq', 200);
        asset(handle, 'DOC_HowTo', 'Setup.md').its('status').should('eq', 200);
    });

    it('drops the chosen package when a new choice is not a ZIP', () => {
        const handle = `it-wizard-bad-choice-${uid}`;
        openCreate(handle);
        toImport();
        cy.get('#wz-content-input').selectFile(contentPackage(), { force: true });
        cy.get('#wz-content-chip').should('have.class', 'visible');

        cy.get('#wz-content-input').selectFile(docFile('notes.txt'), { force: true });
        cy.get('#wz-content-error').should('have.text', 'Unsupported file type. Accepted: .zip');
        cy.get('#wz-content-chip').should('not.have.class', 'visible');
        save();
        saved();
        asset(handle, 'DOC_Other', 'Guide.md').its('status').should('eq', 404);
    });

    it('locks the page while the save runs', () => {
        const handle = `it-wizard-lock-${uid}`;
        cy.login();
        cy.seedApi({ id: handle, name: `Wizard ${handle}` });
        handles.push(handle);

        openEdit(handle);
        toImport();
        cy.get('#wz-content-input').selectFile(contentPackage(), { force: true });
        cy.intercept('PUT', `**/api/v0.9/apis/${handle}`, (req) => req.on('response', (res) => res.setDelay(1000)));
        save();
        cy.get('#cfg-replace-content-confirm').click();
        cy.get('.cfg-layout').should('have.prop', 'inert', true);
        saved();
        asset(handle, 'DOC_Other', 'Guide.md').its('status').should('eq', 200);
    });

    it('shows a removed thumbnail as gone when the import after it is rejected', () => {
        const handle = `it-wizard-thumb-rejected-${uid}`;
        cy.login();
        cy.seedApi({ id: handle, name: `Wizard ${handle}` });
        handles.push(handle);

        openSettings();
        cy.get(`.cfg-edit-trigger[data-id="${handle}"]`).first().click({ force: true });
        cy.get('#wz-thumb-input').selectFile(JPEG, { force: true });
        toAssets();
        save();
        saved();

        openSettings();
        cy.get(`.cfg-edit-trigger[data-id="${handle}"]`).first().click({ force: true });
        cy.get('#wz-thumb-remove').click();
        toAssets();
        toImport();
        cy.get('#wz-content-input').selectFile(invalidContentPackage(), { force: true });
        save();
        cy.get('#cfg-replace-content-confirm').click();

        cy.get('#wz-content-chip').should('contain', 'Not uploaded');
        cy.get('#wz-thumb-name').should('have.text', 'No thumbnail');
        cy.get('#wz-thumb-tag').should('have.attr', 'hidden');
        cy.get('#wz-thumb-undo').should('have.attr', 'hidden');
        asset(handle, 'IMAGE', 'api-icon.jpg').its('status').should('eq', 404);
    });

    it('replaces only the Other document of a name and keeps it in other categories', () => {
        const handle = `it-wizard-doc-category-${uid}`;
        cy.login();
        cy.seedApi({ id: handle, name: `Wizard ${handle}` });
        handles.push(handle);

        openEdit(handle);
        toImport();
        cy.get('#wz-content-input').selectFile({
            contents: createZip([
                { name: 'docs/Guide.md', content: '# Other' },
                { name: 'docs/HowTo/Guide.md', content: '# HowTo' },
            ]),
            fileName: 'content.zip',
            mimeType: 'application/zip',
        }, { force: true });
        save();
        cy.get('#cfg-replace-content-confirm').click();
        saved();

        openEdit(handle);
        cy.contains('#wz-existing-docs-list .cfg-docs-item', 'HowTo/Guide.md').should('exist');
        cy.get('#wz-docs-input').selectFile(docFile('Guide.md', '# New'), { force: true });
        cy.get('#wz-docs-summary').should('have.text', 'When you save: 1 document will be replaced.');
        cy.contains('#wz-existing-docs-list .cfg-docs-item', 'HowTo/Guide.md').should('exist');
        save();
        saved();
        asset(handle, 'DOC_Other', 'Guide.md').its('body').should('contain', 'New');
        asset(handle, 'DOC_HowTo', 'Guide.md').its('body').should('contain', 'HowTo');

        openEdit(handle);
        cy.contains('#wz-existing-docs-list .cfg-docs-item', 'HowTo/Guide.md').find('.cfg-docs-remove').click();
        save();
        saved();
        asset(handle, 'DOC_HowTo', 'Guide.md').its('status').should('eq', 404);
        asset(handle, 'DOC_Other', 'Guide.md').its('status').should('eq', 200);
    });
});
