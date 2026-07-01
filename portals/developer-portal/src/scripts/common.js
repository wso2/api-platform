// Devportal API URL builder. The base segment and version are injected by
// the server (window.__DEVPORTAL_API__, set in the layout); the fallback keeps
// pages working if the global is ever missing. Defined synchronously (outside
// DOMContentLoaded) so it is available before any page script's handlers run.
(function () {
    var cfg = window.__DEVPORTAL_API__ || { base: 'devportal', version: 'v1' };
    window.devportalApi = {
        // Devportal API resource: org('/subscriptions') => '/devportal/v1/subscriptions'
        org: function (path) {
            return '/' + cfg.base + '/' + cfg.version + (path || '');
        },
        // Root resource: root('/organizations') => '/organizations'
        root: function (path) {
            return path || '/';
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
    
    
    // Function to show loading state on subscription button
    window.showSubscribeButtonLoading = function(button) {
        if (button) {
            if (!button.dataset.originalText) {
                button.dataset.originalText = button.innerHTML;
            }
            button.disabled = true;

            const trimmed = (button.textContent || '').trim();
            if (trimmed === 'Subscribe') {
                button.textContent = 'Subscribing...';
            } else if (trimmed === 'Update') {
                button.textContent = 'Updating...';
            }
        }
    };

    // Function to restore subscription button state
    window.resetSubscribeButtonState = function(button) {
        if (button && button.dataset.originalText) {
            button.innerHTML = button.dataset.originalText;
            button.disabled = false;
            delete button.dataset.originalText;
        }
    };

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

        // Remove active class from all links
        navLinks.forEach(link => link.classList.remove('active'));

        // Set the active class based on path
        if (currentPath.endsWith('/') || currentPath === '') {
            document.getElementById('home')?.classList.add('active');
            apiSubmenu.classList.remove('show');
            apisLink?.classList.remove('has-active-submenu');
        } else if (currentPath.includes('/apis')) {
            apisLink?.classList.add('active');
            apiSubmenu.classList.remove('show');
            apisLink?.classList.remove('has-active-submenu');
        } else if (currentPath.includes('/api/')) {
            apiSubmenu.classList.add('show');
            apisLink?.classList.add('active');
            apisLink?.classList.add('has-active-submenu');

            // Extract API ID from URL path and update submenu links
            const apiIdMatch = currentPath.match(/\/api\/([^\/]+)/);
            if (apiIdMatch && apiIdMatch[1]) {
                const apiId = apiIdMatch[1];

                // Update the submenu links with the correct API ID and base path
                document.getElementById('api-overview').href = `${basePath}/api/${apiId}`;
                document.getElementById('api-docs').href = `${basePath}/api/${apiId}/docs/specification`;
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
        } else if (currentPath.includes('/applications/') || currentPath.includes('/applications')) {
            applicationsLink?.classList.add('active');
        } else if (currentPath.includes('/mcps')) {
            document.getElementById('mcps')?.classList.add('active');
            mcpSubmenu.classList.remove('show');
            mcpLink?.classList.remove('has-active-submenu');
        } else if (currentPath.includes('/mcp/')) {
            mcpSubmenu.classList.add('show');
            mcpLink?.classList.add('active');
            mcpLink?.classList.add('has-active-submenu');

            // Extract API ID from URL path and update submenu links
            const apiIdMatch = currentPath.match(/\/mcp\/([^\/]+)/);
            if (apiIdMatch && apiIdMatch[1]) {
                const apiId = apiIdMatch[1];

                // Update the submenu links with the correct API ID and base path
                document.getElementById('mcp-overview').href = `${basePath}/mcp/${apiId}`;
                document.getElementById('mcp-docs').href = `${basePath}/mcp/${apiId}/docs/specification`;

                // Set active submenu item
                if (currentPath.includes('/docs')) {
                    document.getElementById('mcp-docs')?.classList.add('active');
                } else {
                    document.getElementById('mcp-overview')?.classList.add('active');
                }
            }
        } else if (currentPath.includes('/subscriptions')) {
            document.getElementById('subscriptions')?.classList.add('active');
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

    // Copy MCP Server Config JSON to clipboard
    window.copyServerConfig = async function(apiId) {
        const preBlock = document.getElementById(`server-config-${apiId}`);
        const buttonElement = preBlock.nextElementSibling;
        const iconElement = buttonElement.querySelector('i');

        try {
            const text = preBlock.innerText.trim();
            await navigator.clipboard.writeText(text);

            iconElement.classList.remove('bi-clipboard');
            iconElement.classList.add('bi-clipboard-check');
            await showAlert('Server config copied to clipboard!', `default`);

            setTimeout(() => {
                iconElement.classList.remove('bi-clipboard-check');
                iconElement.classList.add('bi-clipboard');
            }, 1500);
        } catch (err) {
            console.error('Failed to copy server config:', err);
            await showAlert('Failed to copy server config', true);
        }
    };
    
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
    
    // Helper function to show message on an API card or subscription card
    window.showApiMessage = function(overlay, message, type = 'success') {
        if (overlay) {
            // Clear any existing auto-hide timers
            if (overlay.hideTimer) {
                clearTimeout(overlay.hideTimer);
                overlay.hideTimer = null;
            }
            
            // Set message - keeping it simple and concise
            const messageText = overlay.querySelector('.message-text');
            if (messageText) messageText.textContent = message;
            
            // Set type (success/error)
            overlay.classList.remove('success', 'error');
            overlay.classList.add(type);
            
            // Update icon - ensure proper class structure for alignment
            const icon = overlay.querySelector('.message-icon');
            if (icon) {
                icon.className = 'bi message-icon ' + type;
                icon.classList.add(type === 'success' ? 'bi-check-circle-fill' : 'bi-exclamation-circle-fill');
            }
            
            // Show the overlay (remove hidden class if it exists)
            overlay.classList.remove('hidden');
            
            // Auto-hide after the designated time only for success messages
            // Error messages remain visible until user closes them manually
            if (type === 'success') {
                overlay.hideTimer = setTimeout(() => {
                    overlay.classList.add('hidden');
                }, 5000);
            }
            
            return overlay;
        }
        return null;
    };

    // Toggle accordion chevron icons
    document.querySelectorAll('.accordion-header').forEach(btn => {
        btn.addEventListener('click', function () {
            const icon = this.querySelector('.chevron-icon');
            if (icon) {
                icon.classList.toggle('bi-chevron-down');
                icon.classList.toggle('bi-chevron-up');
            }
        });
    });

    // Load image vectors and apply theme colors
    let primaryMain = getComputedStyle(document.documentElement).getPropertyValue("--primary-main-color").trim();
    let primaryLight = getComputedStyle(document.documentElement).getPropertyValue("--primary-light-color").trim();
    let primaryLightest = getComputedStyle(document.documentElement).getPropertyValue("--primary-lightest-color").trim();
    let secondaryMain = getComputedStyle(document.documentElement).getPropertyValue("--secondary-main-color").trim();

    const apisImage = document.getElementById("apisImage");
    const launchImage = document.getElementById("launchImage");
    const heroImage = document.getElementById("heroImage");
    const applicationsImage = document.getElementById("applicationsImage");

    if (apisImage) {
        fetch(document.querySelector("#apisImage img").src)
            .then(response => response.text())
            .then(data => {
                apisImage.innerHTML = data;
                apisImage.querySelectorAll("#primaryMain").forEach(el => {
                    el.setAttribute("fill", primaryMain);

                });
                apisImage.querySelectorAll("#primaryLight").forEach(el => {
                    el.setAttribute("fill", primaryLight);

                });
                apisImage.querySelectorAll("#primaryLightest").forEach(el => {
                    el.setAttribute("fill", primaryLightest);

                });
                apisImage.querySelectorAll("#secondaryMain").forEach(el => {
                    el.setAttribute("fill", secondaryMain);

                });
            });
    }

    if (applicationsImage) {
        fetch(document.querySelector("#applicationsImage img").src)
            .then(response => response.text())
            .then(data => {
                applicationsImage.innerHTML = data;
                applicationsImage.querySelectorAll("#primaryMain").forEach(el => {
                    el.setAttribute("fill", primaryMain);
                });
                applicationsImage.querySelectorAll("#primaryLight").forEach(el => {
                    el.setAttribute("fill", primaryLight);
                });
                applicationsImage.querySelectorAll("#primaryLightest").forEach(el => {
                    el.setAttribute("fill", primaryLightest);
                });
                applicationsImage.querySelectorAll("#secondaryMain").forEach(el => {
                    el.setAttribute("fill", secondaryMain);
                });
            });
    }

    if (launchImage) {
        fetch(document.querySelector("#launchImage img").src)
            .then(response => response.text())
            .then(data => {
                launchImage.innerHTML = data;
                launchImage.querySelectorAll("#primaryMain").forEach(el => {
                    el.setAttribute("fill", primaryMain);

                });
                launchImage.querySelectorAll("#primaryLight").forEach(el => {
                    el.setAttribute("fill", primaryLight);

                });
                launchImage.querySelectorAll("#primaryLightest").forEach(el => {
                    el.setAttribute("fill", primaryLightest);

                });
                launchImage.querySelectorAll("#secondaryMain").forEach(el => {
                    el.setAttribute("fill", secondaryMain);
                });
            });
    }

    if (heroImage) {
        fetch(document.querySelector("#heroImage img").src)
            .then(response => response.text())
            .then(data => {
                heroImage.innerHTML = data;
                heroImage.querySelectorAll("#primaryMain").forEach(el => {
                    el.setAttribute("stop-color", primaryLightest);
                });
                heroImage.querySelectorAll("#primaryLight").forEach(el => {
                    el.setAttribute("stop-color", primaryLight);
                });
                heroImage.querySelectorAll("#primaryLightest").forEach(el => {
                    el.setAttribute("stop-color", primaryLightest);
                });
                heroImage.querySelectorAll("#secondaryMain").forEach(el => {
                    el.setAttribute("stop-color", secondaryMain);
                });
            });
    }

});

