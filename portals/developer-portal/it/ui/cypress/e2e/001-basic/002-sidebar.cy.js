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

// Sidebar behaviour. The sidebar is collapsed by default; the expand/collapse
// button (#collapseBtn) pins it open (class `expanded`) or force-collapses it
// (class `force-collapse`), persisting the choice in localStorage.
//
// NOTE: hover-to-open / stay-while-inside / close-on-leave are driven purely by
// CSS `:hover` (.sidebar:hover:not(.force-collapse)), which cannot be triggered
// by Cypress without the cypress-real-events plugin. This suite covers the
// JS/localStorage-driven behaviour instead: collapsed-by-default, the nav items,
// pin-to-open via the button, persistence across navigation + reload, and
// collapse via the button.

describe('Developer Portal — Sidebar', () => {
    beforeEach(() => {
        cy.on('uncaught:exception', () => false);
        cy.visitPortal(); // portal home
    });

    it('is collapsed by default and lists the navigation items', () => {
        cy.get('#sidebar')
            .should('be.visible')
            .and('not.have.class', 'expanded');
        // Collapsed width (~4.625rem).
        cy.get('#sidebar').invoke('outerWidth').should('be.lessThan', 140);
        cy.get('#collapseBtn .collapse-text').should('contain', 'Expand');

        // Navigation items are present.
        cy.get('#sidebar #home').should('exist');
        cy.get('#sidebar #apis').should('exist');
        cy.get('#sidebar #mcps').should('exist');
        cy.get('#sidebar #api-workflows').should('exist');
        cy.get('#sidebar #applications').should('exist');
    });

    it('pins open via the expand button and stays open across navigation and reload', () => {
        // Click expand → the sidebar is pinned open (independent of the mouse).
        cy.get('#collapseBtn').click();
        cy.get('#sidebar').should('have.class', 'expanded');
        cy.get('#collapseBtn .collapse-text').should('contain', 'Collapse');
        cy.get('#sidebar').invoke('outerWidth').should('be.greaterThan', 180);

        // Navigate to APIs — the pinned state persists (localStorage) ...
        cy.get('#sidebar #apis').click();
        cy.url().should('include', '/apis');
        cy.get('#sidebar').should('have.class', 'expanded');

        // ... and survives a full reload.
        cy.reload();
        cy.get('#sidebar').should('have.class', 'expanded');
    });

    it('collapses via the collapse button', () => {
        // Pin open first.
        cy.get('#collapseBtn').click();
        cy.get('#sidebar').should('have.class', 'expanded');

        // Click again → force-collapsed (stays collapsed even on hover).
        cy.get('#collapseBtn').click();
        cy.get('#sidebar')
            .should('not.have.class', 'expanded')
            .and('have.class', 'force-collapse');
        cy.get('#collapseBtn .collapse-text').should('contain', 'Expand');
        cy.get('#sidebar').invoke('outerWidth').should('be.lessThan', 140);
    });
});
