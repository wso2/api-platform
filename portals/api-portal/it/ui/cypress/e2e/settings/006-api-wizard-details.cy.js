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

// The API wizard's Details step (cards, thumbnail picker) and Spec step. The thumbnail is
// sent with the API save as the `thumbnail` part or `removeThumbnail: true`.

describe('Settings — API wizard Details and Spec steps', () => {
    const uid = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const handles = [];
    const settingsUrl = () => `${Cypress.env('BASE_PATH')}/${Cypress.env('ORG_HANDLE')}/settings`;
    const PNG = {
        contents: Cypress.Buffer.from('89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c63606060f80f00010401005fe5c34b0000000049454e44ae426082', 'hex'),
        fileName: 'logo.png',
        mimeType: 'image/png',
    };
    const JPEG = { contents: Cypress.Buffer.from('ffd8ffe000104a46494600010100000100010000ffd9', 'hex'), fileName: 'photo.jpg', mimeType: 'image/jpeg' };
    const SPEC = JSON.stringify({ openapi: '3.0.3', info: { title: 'Wizard', version: '1.0.0' }, paths: {} });

    const image = (handle, fileName) => cy.apiRequest('GET',
        `/api/v0.9/apis/${handle}/assets?type=IMAGE&fileName=${fileName}`, { failOnStatusCode: false });
    const openSettings = () => {
        cy.login();
        cy.visit(settingsUrl());
        cy.get('.cfg-nav-item[data-panel="cfg-apis"]').click();
    };
    const openEdit = (handle) => {
        openSettings();
        cy.get(`.cfg-edit-trigger[data-id="${handle}"]`).first().click({ force: true });
        cy.get('#cfg-wstep-0').should('be.visible');
    };
    const seed = (name) => {
        const handle = `it-wizard-${name}-${uid}`;
        cy.login();
        cy.seedApi({ id: handle, name: `Wizard ${handle}` });
        handles.push(handle);
        return handle;
    };
    const saveFromDetails = () => {
        cy.get('#cfg-wizard-next').click();
        cy.get('#cfg-wizard-next').click();
        cy.get('#cfg-wizard-next').click();
    };
    const saved = () => cy.get('#cfg-apis-wizard', { timeout: 20000 }).should('not.be.visible');
    const multipartText = (body) => (typeof body === 'string' ? body : new TextDecoder().decode(body));

    after(() => {
        cy.login();
        handles.forEach((handle) => cy.deleteApi(handle));
    });

    it('groups the details into cards, with the thumbnail row first in General', () => {
        openSettings();
        cy.get('#cfg-add-api-btn').click();
        cy.get('#cfg-wizard-title').should('have.text', 'Create API');
        cy.get('#cfg-wstep-0 .cfg-form-card-title').then(($titles) => {
            expect([...$titles].map((el) => el.textContent.trim()))
                .to.deep.eq(['General', 'Endpoints', 'Labels and tags', 'Ownership', 'Subscription plans']);
        });
        cy.get('#wz-card-general .cfg-form-group').first().find('#wz-thumb-row').should('exist');
        cy.get('#wz-card-general').find('#wz-status').should('exist');

        cy.get('#wz-thumb-preview').should('have.class', 'cfg-api-avatar');
        cy.get('#wz-thumb-name').should('have.text', 'No thumbnail');
        cy.get('#wz-thumb-hint').should('have.text', 'PNG or JPG');
        cy.get('#wz-thumb-input').should('have.attr', 'accept', '.png,.jpg,.jpeg');
        cy.get('#wz-thumb-input').focus();
        cy.focused().should('have.id', 'wz-thumb-input');
        cy.get('#wz-thumb-remove').should('not.be.visible');

        cy.get('#wz-name').type('Pets API');
        cy.get('#wz-version').type('v1.0');
        cy.get('#cfg-wizard-subtitle').should('have.text', 'Pets API · v1.0');
        cy.get('#wz-thumb-preview').should('have.text', 'PA');
    });

    it('requires an API type on create and saves an API with no production URL', () => {
        const handle = `it-wizard-no-prod-${uid}`;
        openSettings();
        cy.get('#cfg-add-api-btn').click();
        cy.get('label[for="wz-type"] .required').should('have.text', '*');
        cy.get('label[for="wz-prod"] .required').should('not.exist');
        cy.get('#wz-type').should(([select]) => expect(select.value).to.eq(''));
        cy.get('#wz-type option:selected').should('have.text', 'Select a type');
        cy.get('#wz-status').should('have.value', 'PUBLISHED');
        cy.get('#wz-name').type('Type Wizard');
        cy.get('#wz-version').type('v1.0');
        cy.get('#wz-handle').clear().type(handle);

        cy.get('#cfg-wizard-next').click();
        cy.get('#wz-type-error').scrollIntoView().should('be.visible').and('have.text', 'API type is required.');
        cy.get('#wz-prod-error').should('not.be.visible');
        cy.get('#cfg-wstep-0').should('be.visible');

        cy.get('#wz-type').select('RestApi');
        cy.get('#wz-type-error').should('not.be.visible');
        cy.get('#cfg-wizard-next').click();
        cy.get('#cfg-wstep-1').should('be.visible');

        cy.get('#wz-spec-input').selectFile({ contents: Cypress.Buffer.from(SPEC), fileName: 'definition.json' }, { force: true });
        cy.get('#cfg-wizard-next').click();
        cy.get('#cfg-wstep-2').scrollIntoView().should('be.visible');
        handles.push(handle);
        cy.get('#cfg-wizard-next').click();
        saved();
        cy.apiRequest('GET', `/api/v0.9/apis/${handle}`).then(({ status, body }) => {
            expect(status).to.eq(200);
            expect(body.endPoints?.productionURL || '').to.eq('');
        });
    });

    it('saves a picked thumbnail, then replaces it, sending only the last choice', () => {
        const handle = seed('thumb');

        openEdit(handle);
        cy.get('#wz-thumb-input').selectFile(PNG, { force: true });
        cy.get('#wz-thumb-preview img').should('exist');
        cy.get('#wz-thumb-name').should('have.text', 'logo.png');
        cy.get('#wz-thumb-tag').should('be.visible').and('have.text', 'Replaced when you save');
        saveFromDetails();
        saved();
        image(handle, 'api-icon.png').its('status').should('eq', 200);

        openSettings();
        cy.get(`#cfg-api-row-${handle} img.cfg-api-avatar--img`)
            .should('have.attr', 'src').and('contain', `/apis/${handle}/assets`).and('contain', 'fileName=api-icon.png');
        cy.get(`#cfg-api-row-${handle} img.cfg-api-avatar--img`).should(($img) => expect($img[0].naturalWidth).to.be.greaterThan(0));

        openEdit(handle);
        cy.get('#wz-thumb-name').should('have.text', 'api-icon.png');
        cy.get('#wz-thumb-preview img').should('have.attr', 'src').and('contain', 'fileName=api-icon.png');
        cy.get('#wz-thumb-preview img').should(($img) => expect($img[0].naturalWidth).to.be.greaterThan(0));
        cy.get('#wz-thumb-tag').should('not.be.visible');

        cy.get('#wz-thumb-remove').should('be.visible').click();
        cy.get('#wz-thumb-tag').should('have.text', 'Removed when you save');
        cy.get('#wz-thumb-preview').should('have.class', 'cfg-api-avatar');
        cy.get('#wz-thumb-undo').should('be.visible').click();
        cy.get('#wz-thumb-tag').should('not.be.visible');
        cy.get('#wz-thumb-preview img').should('exist');

        cy.get('#wz-thumb-remove').click();
        cy.get('#wz-thumb-input').selectFile(JPEG, { force: true });
        cy.get('#wz-thumb-tag').should('have.text', 'Replaced when you save');

        // Sending both fields would be a 400, so a saved JPEG shows only the replace was sent.
        saveFromDetails();
        saved();
        image(handle, 'api-icon.jpg').its('status').should('eq', 200);
        image(handle, 'api-icon.png').its('status').should('eq', 404);

        cy.visitPortal('/apis');
        cy.contains('.api-card', `Wizard ${handle}`).find('img.api-card-img')
            .should('have.attr', 'src').and('contain', 'fileName=api-icon.jpg');
    });

    it('removes the thumbnail on save', () => {
        const handle = seed('thumb-remove');
        openEdit(handle);
        cy.get('#wz-thumb-input').selectFile(PNG, { force: true });
        saveFromDetails();
        saved();

        openEdit(handle);
        cy.get('#wz-thumb-remove').click();
        cy.intercept('PUT', `**/api/v0.9/apis/${handle}`).as('save');
        saveFromDetails();
        cy.wait('@save').then(({ request }) => {
            const body = multipartText(request.body);
            expect(body).to.contain('"removeThumbnail":true');
            expect(body).not.to.contain('name="thumbnail"');
        });
        saved();
        image(handle, 'api-icon.png').its('status').should('eq', 404);

        openSettings();
        cy.get(`#cfg-api-row-${handle} img.cfg-api-avatar--img`).should('not.exist');
        cy.get(`#cfg-api-row-${handle} span.cfg-api-avatar`).should('have.text', 'WI');
    });

    it('shows the server message under the thumbnail row for a rejected file', () => {
        const handle = seed('thumb-bad');
        openEdit(handle);
        cy.get('#wz-thumb-input').selectFile({ contents: Cypress.Buffer.from('not an image'), fileName: 'logo.png', mimeType: 'image/png' }, { force: true });
        saveFromDetails();

        cy.get('#wz-thumb-error').should('be.visible').and('have.text', 'Thumbnail must be a PNG or JPEG image.');
        cy.get('#cfg-wstep-0').should('be.visible');
        image(handle, 'api-icon.png').its('status').should('eq', 404);
    });

    it('requires a spec on create and accepts only the file types of the API type', () => {
        openSettings();
        cy.get('#cfg-add-api-btn').click();
        cy.get('#wz-name').type('Spec Wizard');
        cy.get('#wz-version').type('v1.0');
        cy.get('#wz-prod').type('https://backend.example.invalid/pets');
        cy.get('#wz-type').select('GRAPHQL');
        cy.get('#cfg-wizard-next').click();

        cy.get('#wz-spec-desc').should('have.text', 'The contract that defines this API. Required.');
        cy.get('#wz-spec-onfile').should('not.be.visible');
        cy.get('#wz-spec-formats').should('be.visible').and('contain', 'Accepted formats');
        cy.get('#wz-spec-format-name').should('have.text', 'GraphQL SDL');
        cy.get('#wz-spec-format-exts').should('have.text', '.graphql .gql');
        cy.get('#wz-spec-input').should('have.attr', 'accept', '.graphql,.gql');
        cy.get('#cfg-wizard-next').click();
        cy.get('#wz-spec-error').should('be.visible');
        cy.get('#cfg-wstep-1').should('be.visible');

        cy.get('#wz-spec-input').selectFile({ contents: Cypress.Buffer.from(SPEC), fileName: 'definition.json' }, { force: true });
        cy.get('#alertToast').should('have.class', 'error')
            .find('.alert-toast-message').should('have.text', 'The GraphQL SDL spec must be a .graphql or .gql file.');
        cy.get('#wz-spec-chip').should('not.be.visible');

        cy.get('#wz-spec-input').selectFile({ contents: Cypress.Buffer.from('type Query { ping: String }'), fileName: 'schema.graphql' }, { force: true });
        cy.get('#wz-spec-chip').scrollIntoView().should('be.visible').and('contain', 'schema.graphql').and('contain', 'GraphQL SDL · uploaded when you save');

        cy.get('#cfg-wizard-back-step').click();
        cy.get('#wz-type').select('WS');
        cy.get('#cfg-wizard-next').click();
        cy.get('#wz-spec-chip').should('not.be.visible');
        cy.get('#wz-spec-format-name').should('have.text', 'AsyncAPI');
        cy.get('#wz-spec-format-exts').should('have.text', '.json .yaml .yml');
        cy.get('#wz-spec-input').should('have.attr', 'accept', '.json,.yaml,.yml');
        cy.get('#wz-spec-input').selectFile({ contents: Cypress.Buffer.from(SPEC), fileName: 'definition.json' }, { force: true });
        cy.get('#wz-spec-chip').should('contain', 'AsyncAPI · uploaded when you save');

        cy.get('#wz-spec-remove').click();
        cy.get('#wz-spec-chip').should('not.be.visible');
    });

    it('shows the current spec when editing and marks it when a new file is chosen', () => {
        const handle = seed('spec');
        openEdit(handle);
        cy.get('#cfg-wizard-title').should('have.text', 'Edit API');
        cy.get('#cfg-wizard-subtitle').should('have.text', `Wizard ${handle} · v1.0`);
        cy.get('#cfg-wizard-next').click();

        cy.get('#wz-spec-desc').should('have.text', 'Upload a new spec to replace the current one.');
        cy.get('#wz-spec-onfile').should('be.visible').and('contain', 'Current spec').and('contain', 'On file for this API');
        cy.get('#wz-spec-replaced-tag').should('not.be.visible');

        cy.get('#wz-spec-input').selectFile({ contents: Cypress.Buffer.from(SPEC), fileName: 'definition.json' }, { force: true });
        cy.get('#wz-spec-replaced-tag').scrollIntoView().should('be.visible').and('have.text', 'Will be replaced');
        cy.get('#wz-spec-chip').should('contain', 'OpenAPI · uploaded when you save');

        cy.get('#wz-spec-remove').click();
        cy.get('#wz-spec-replaced-tag').should('not.be.visible');
        cy.get('#cfg-wizard-next').click();
        cy.get('#cfg-wstep-2').scrollIntoView().should('be.visible');
    });
});
