// API Portal API URL builder. The base segment and version are injected by
// the server (window.__API_PORTAL_API__, set in the layout); the fallback keeps
// pages working if the global is ever missing. Defined synchronously (outside
// DOMContentLoaded) so it is available before any page script's handlers run.
(function () {
    var cfg = window.__API_PORTAL_API__ || { base: 'api', version: 'v0.9' };
    window.apiPortalApi = {
        // API Portal API resource under the versioned base:
        // root('/subscriptions') => '/api/v0.9/subscriptions'
        root: function (path) {
            return '/' + cfg.base + '/' + cfg.version + (path || '');
        },
        // Per-session CSRF token from the XSRF-TOKEN cookie, to send as
        // X-CSRF-Token on mutating requests (see csrfProtection middleware).
        csrfToken: function () {
            var m = document.cookie.match(/(?:^|;\s*)XSRF-TOKEN=([^;]+)/);
            return m ? decodeURIComponent(m[1]) : '';
        },
    };
})();

document.addEventListener("DOMContentLoaded", function () {
    const sidebar = document.getElementById('sidebar');
    const collapseBtn = document.getElementById('collapseBtn');

    // Remove reference to sidebarPlaceholder which no longer exists
    const sidebarPlaceholder = document.getElementById('sidebarPlaceholder');
    if (sidebarPlaceholder) {
        sidebarPlaceholder.remove();
    }

    // Restore persisted sidebar state
    if (localStorage.getItem('sidebar-expanded') === '1') {
        sidebar.classList.add('expanded');
        sidebar.classList.remove('force-collapse');
        collapseBtn.querySelector('.collapse-text').textContent = "Collapse";
    }

    // Track if the mouse has left the sidebar
    let mouseLeftSidebar = false;

    // Add mouse enter and leave event listeners
    sidebar.addEventListener('mouseleave', () => {
        mouseLeftSidebar = true;
    });

    sidebar.addEventListener('mouseenter', () => {
        // If mouse re-enters and the sidebar was previously force-collapsed
        if (mouseLeftSidebar && sidebar.classList.contains('force-collapse')) {
            sidebar.classList.remove('force-collapse');
        }
        mouseLeftSidebar = false;
    });

    // Toggle between expanded and collapsed state when clicking the collapse button
    collapseBtn.addEventListener('click', () => {
        if (sidebar.classList.contains('expanded')) {
            // If currently expanded (pinned), collapse and prevent hover expansion
            sidebar.classList.remove('expanded');
            sidebar.classList.add('force-collapse');
            collapseBtn.querySelector('.collapse-text').textContent = "Expand";
            localStorage.setItem('sidebar-expanded', '0');
        } else {
            // If currently collapsed, pin it expanded
            sidebar.classList.add('expanded');
            sidebar.classList.remove('force-collapse');
            collapseBtn.querySelector('.collapse-text').textContent = "Collapse";
            localStorage.setItem('sidebar-expanded', '1');
        }
    });

    /* ── Mobile drawer ──
       Below 860px the sidebar is off-canvas (see side-bar.css) and this is the only
       way to reach it. Everything below is a no-op above that breakpoint, where the
       toggle and backdrop are display:none and never receive events. */
    const sidebarToggle = document.getElementById('sidebarToggle');
    const sidebarBackdrop = document.getElementById('sidebarBackdrop');

    if (sidebarToggle && sidebarBackdrop) {
        sidebarBackdrop.hidden = false; // inert via CSS until the breakpoint applies

        /* matchMedia mirrors side-bar.css's own 860px so the two can't disagree.
           Declared before the helpers below because they read it. */
        const mobileQuery = window.matchMedia('(max-width: 860px)');

        const isOpen = () => sidebar.classList.contains('mobile-open');

        /* A closed drawer is only moved off-screen by a transform, so it stayed in the
           tab order and the accessibility tree: 13 focusable links a keyboard user
           could tab into invisibly, and that a screen reader would still announce.
           `inert` removes both; `aria-hidden` is set alongside for assistive tech that
           predates it. Above the breakpoint the sidebar is ordinary visible layout, so
           neither may ever apply there — which is why this is re-run on breakpoint
           change and not just on open/close. */
        const syncDrawerHiddenState = () => {
            const hidden = mobileQuery.matches && !isOpen();
            sidebar.toggleAttribute('inert', hidden);
            sidebar.setAttribute('aria-hidden', hidden ? 'true' : 'false');
        };

        const openDrawer = () => {
            sidebar.classList.add('mobile-open');
            sidebarBackdrop.classList.add('visible');
            sidebarToggle.setAttribute('aria-expanded', 'true');
            // Stops the page scrolling behind the drawer on touch. A class, not an
            // inline style, so the rule can live inside side-bar.css's 860px media
            // query and lapse on its own above the breakpoint.
            document.body.classList.add('drawer-open');
            // Before any caller focuses into the drawer — focus() is a no-op on an
            // inert subtree.
            syncDrawerHiddenState();
        };

        const closeDrawer = () => {
            /* Move focus out before the subtree goes inert, otherwise it lands on
               <body> and the user loses their place. The toggle is where they came
               from, so it is where they go back to. */
            if (sidebar.contains(document.activeElement)) {
                sidebarToggle.focus();
            }
            sidebar.classList.remove('mobile-open');
            sidebarBackdrop.classList.remove('visible');
            sidebarToggle.setAttribute('aria-expanded', 'false');
            document.body.classList.remove('drawer-open');
            syncDrawerHiddenState();
        };

        sidebarToggle.addEventListener('click', () => {
            if (isOpen()) {
                closeDrawer();
            } else {
                openDrawer();
                sidebar.querySelector('.nav-link')?.focus();
            }
        });

        sidebarBackdrop.addEventListener('click', closeDrawer);

        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && isOpen()) {
                closeDrawer();
                sidebarToggle.focus();
            }
        });

        /* Navigating leaves the drawer open over the new page on a same-document
           route (the submenu links are '#'-href and handled in JS), so close on any
           destination pick. Submenu parents only expand a section — those stay open. */
        sidebar.addEventListener('click', (e) => {
            const link = e.target.closest('a.nav-link');
            if (!link || !isOpen()) return;
            const href = link.getAttribute('href');
            if (!href || href === '#') return; // section expander, not a destination
            closeDrawer();
        });

        /* Rotating to landscape / resizing past the breakpoint must not leave the
           drawer flagged open, nor leave the sidebar inert once it is part of the
           desktop layout again. matchMedia rather than a resize listener: it fires
           exactly on the breakpoint crossing, including orientation changes, which
           don't always emit resize. */
        const syncToBreakpoint = () => {
            if (!mobileQuery.matches && isOpen()) {
                closeDrawer(); // already re-syncs the hidden state
            } else {
                syncDrawerHiddenState();
            }
        };
        if (mobileQuery.addEventListener) {
            mobileQuery.addEventListener('change', syncToBreakpoint);
        } else if (mobileQuery.addListener) {
            mobileQuery.addListener(syncToBreakpoint); // Safari < 14
        }

        // Initial state: inert below the breakpoint (drawer starts closed), never above.
        syncDrawerHiddenState();
    }

    // Set active status based on current URL path
    const setActiveSidebarLink = () => {
        const currentPath = window.location.pathname;
        const navLinks = document.querySelectorAll('.nav-link');
        const apiSubmenu = document.getElementById('api-submenu');

        const mcpSubmenu = document.getElementById('mcp-submenu');
        const apisLink = document.getElementById('apis');
        const applicationsLink = document.getElementById('applications');
        const mcpLink = document.getElementById('mcps');

        // Function to extract base path from links in the sidebar
        const extractBasePath = () => {
            const homeLink = document.getElementById('home');
            if (homeLink && homeLink.getAttribute('href')) {
                const href = homeLink.getAttribute('href');
                // Remove trailing slash if present
                return href.endsWith('/') ? href.slice(0, -1) : href;
            }
            return '';
        };

        const basePath = extractBasePath();

        // Resolve the path segment that follows the view-scoped base path so nav
        // matching is exact. e.g. "/org/views/default/api-keys" -> firstSegment "api-keys".
        // Settings is org-scoped (/:org/settings) and does not sit under basePath, so it
        // is handled via a suffix check below.
        let rest = currentPath;
        if (basePath && currentPath.indexOf(basePath) === 0) {
            rest = currentPath.slice(basePath.length);
        }
        const firstSegment = rest.replace(/^\/+/, '').split('/')[0];

        // Remove active class from all links
        navLinks.forEach(link => link.classList.remove('active'));

        // Match on the first path segment. Order the singular API/MCP detail routes
        // (submenu-bearing) before the plural listing routes, and guard optional
        // submenu elements — they are absent in single-mode portals
        // (APIS_ONLY / MCP_SERVERS_ONLY).
        if (firstSegment === '') {
            document.getElementById('home')?.classList.add('active');
            apiSubmenu?.classList.remove('show');
            apisLink?.classList.remove('has-active-submenu');
        } else if (firstSegment === 'api-workflows') {
            document.getElementById('api-workflows')?.classList.add('active');
        } else if (firstSegment === 'api-keys') {
            // Global API Keys page (distinct from the per-API /api/:id/api-keys submenu item)
            document.getElementById('api-keys')?.classList.add('active');
        } else if (firstSegment === 'api') {
            apiSubmenu?.classList.add('show');
            apisLink?.classList.add('active');
            apisLink?.classList.add('has-active-submenu');

            // Extract API ID from URL path and update submenu links
            const apiIdMatch = currentPath.match(/\/api\/([^/]+)/);
            if (apiIdMatch && apiIdMatch[1]) {
                const apiId = apiIdMatch[1];

                // Update the submenu links with the correct API ID and base path
                const overviewLink = document.getElementById('api-overview');
                if (overviewLink) overviewLink.href = `${basePath}/api/${apiId}`;
                const docsLink = document.getElementById('api-docs');
                if (docsLink) docsLink.href = `${basePath}/api/${apiId}/docs/specification`;
                const apiKeysLink = document.getElementById('api-keys-nav');
                if (apiKeysLink) {
                    apiKeysLink.href = `${basePath}/api/${apiId}/api-keys`;
                }

                // Set active submenu item
                if (currentPath.includes('/api-keys')) {
                    document.getElementById('api-keys-nav')?.classList.add('active');
                } else if (currentPath.includes('/docs')) {
                    document.getElementById('api-docs')?.classList.add('active');
                } else {
                    document.getElementById('api-overview')?.classList.add('active');
                }
            }
        } else if (firstSegment === 'apis') {
            apisLink?.classList.add('active');
            apiSubmenu?.classList.remove('show');
            apisLink?.classList.remove('has-active-submenu');
        } else if (firstSegment === 'applications') {
            applicationsLink?.classList.add('active');
        } else if (firstSegment === 'mcp') {
            mcpSubmenu?.classList.add('show');
            mcpLink?.classList.add('active');
            mcpLink?.classList.add('has-active-submenu');

            // Extract API ID from URL path and update submenu links
            const apiIdMatch = currentPath.match(/\/mcp\/([^/]+)/);
            if (apiIdMatch && apiIdMatch[1]) {
                const apiId = apiIdMatch[1];

                // Update the submenu links with the correct API ID and base path
                const mcpOverviewLink = document.getElementById('mcp-overview');
                if (mcpOverviewLink) mcpOverviewLink.href = `${basePath}/mcp/${apiId}`;
                const mcpDocsLink = document.getElementById('mcp-docs');
                if (mcpDocsLink) mcpDocsLink.href = `${basePath}/mcp/${apiId}/docs/specification`;

                // Set active submenu item
                if (currentPath.includes('/docs')) {
                    document.getElementById('mcp-docs')?.classList.add('active');
                } else {
                    document.getElementById('mcp-overview')?.classList.add('active');
                }
            }
        } else if (firstSegment === 'mcps') {
            document.getElementById('mcps')?.classList.add('active');
            mcpSubmenu?.classList.remove('show');
            mcpLink?.classList.remove('has-active-submenu');
        } else if (firstSegment === 'subscriptions') {
            document.getElementById('subscriptions')?.classList.add('active');
        } else if (firstSegment === 'settings' || currentPath.includes('/settings')) {
            document.getElementById('admin-settings')?.classList.add('active');
        }
    };

    // Call the function when page loads
    setActiveSidebarLink();

    // Set active documentation link based on current path
    const setActiveDocLink = () => {
        const currentPath = window.location.pathname;
        const docLinks = document.querySelectorAll('.doc-link');

        // Check if we're on a docs page
        if (currentPath.includes('/docs/')) {
            docLinks.forEach(link => {
                const href = link.getAttribute('href');
                // Remove active class first
                link.classList.remove('active');

                // Add active class if the href matches the current path
                if (href === currentPath ||
                    (href && currentPath.endsWith(href)) ||
                    (href && currentPath === href)) {
                    link.classList.add('active');
                }
            });
        }
    };

    // Call the function when page loads
    setActiveDocLink();

    // Handle API card message overlays
    const messageOverlays = document.querySelectorAll('.message-overlay');
    messageOverlays.forEach(overlay => {
        // Add hidden class initially
        overlay.classList.add('hidden');
        
        // Add click handler to close button
        const closeBtn = overlay.querySelector('.close-message');
        if (closeBtn) {
            closeBtn.addEventListener('click', function() {
                overlay.classList.add('hidden');
            });
        }
    });

});

