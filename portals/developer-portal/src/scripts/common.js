document.addEventListener("DOMContentLoaded", function () {
    const sidebar = document.getElementById('sidebar');
    const collapseBtn = document.getElementById('collapseBtn');
    
    // Remove reference to sidebarPlaceholder which no longer exists
    const sidebarPlaceholder = document.getElementById('sidebarPlaceholder');
    if (sidebarPlaceholder) {
        sidebarPlaceholder.remove();
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
            // If currently expanded, collapse and add a class to prevent hover expansion
            sidebar.classList.remove('expanded');
            
            // Disable hover expansion until mouse re-enters
            sidebar.classList.add('force-collapse');
            
            // Update button text and icon
            collapseBtn.querySelector('.collapse-text').textContent = "Expand";
            collapseBtn.querySelector('i').classList.remove('bi-chevron-left');
            collapseBtn.querySelector('i').classList.add('bi-chevron-right');
        } else {
            // If currently collapsed, expand
            sidebar.classList.add('expanded');
            sidebar.classList.remove('force-collapse');
            
            // Update button text and icon
            collapseBtn.querySelector('.collapse-text').textContent = "Collapse";
            collapseBtn.querySelector('i').classList.remove('bi-chevron-right');
            collapseBtn.querySelector('i').classList.add('bi-chevron-left');
        }
    });
    
    
    // Function to show loading state on subscription button
    window.showSubscribeButtonLoading = function(button) {
        if (button) {
            // Store original text
            button.dataset.originalText = button.innerHTML;
            button.disabled = true;

            if (button.textContent === 'Subscribe') {
                button.textContent = 'Subscribing...';
            } else if (button.textContent === 'Update') {
                button.textContent = 'Updating...';
            }
        }
    };

    // Function to restore subscription button state
    window.resetSubscribeButtonState = function(button) {
        if (button && button.dataset.originalText) {
            button.innerHTML = button.dataset.originalText;
            button.disabled = false;
        }
    };

    // Set active status based on current URL path
    const setActiveSidebarLink = () => {
        const currentPath = window.location.pathname;
        const navLinks = document.querySelectorAll('.nav-link');
        const apiSubmenu = document.getElementById('api-submenu');
        const mcpSubmenu = document.getElementById('mcp-submenu');
        const apisLink = document.getElementById('apis');
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

                // Set active submenu item
                if (currentPath.includes('/docs')) {
                    document.getElementById('api-docs')?.classList.add('active');
                } else {
                    document.getElementById('api-overview')?.classList.add('active');
                }
            }
        } else if (currentPath.includes('/applications')) {
            document.getElementById('applications')?.classList.add('active');
            apiSubmenu.classList.remove('show');
            apisLink?.classList.remove('has-active-submenu');
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

    // Utility function to add a newly created application to all relevant dropdowns
    function addAppToAllDropdowns(appId, appName, sourceType) {
        // Determine what type of dropdowns to update based on source
        const selector = sourceType === 'api-card' ? '.api-card .custom-dropdown' : '.subscription-card .custom-dropdown';
        const dropdowns = document.querySelectorAll(selector);
        
        dropdowns.forEach(dropdown => {
            const selectItemsContainer = dropdown.querySelector('.select-items-container');
            if (!selectItemsContainer) return;
            
            // Create a temporary container to convert HTML string to DOM element
            const tempContainer = document.createElement('div');
            tempContainer.innerHTML = `
                <div class="select-item" role="button" data-value="${appId}" data-app-name="${appName}">
                    <span>${appName}</span>
                    <img src="https://raw.githubusercontent.com/wso2/docs-bijira/refs/heads/main/en/devportal-theming/success-rounded.svg"
                        alt="Subscribed" class="subscription-icon" style="display: none;" />
                </div>
            `;
            
            // Get the actual DOM element
            const newAppItem = tempContainer.firstElementChild;
            
            // Add click event listener to the DOM element
            newAppItem.addEventListener('click', function(e) {
                e.stopPropagation();
                
                // Get the parent card
                const parentCard = dropdown.closest('.api-card') || dropdown.closest('.subscription-card');
                
                // Update hidden input with selected app ID
                const hiddenField = sourceType === 'api-card' 
                    ? document.getElementById(dropdown.querySelector("[id^='selectedAppId-']").id)
                    : dropdown.querySelector('input[type="hidden"]');
                    
                if (hiddenField) {
                    hiddenField.value = appId;
                }
                
                // Update display text
                const selectedText = dropdown.querySelector('.selected-text');
                if (selectedText) {
                    selectedText.textContent = appName;
                    selectedText.classList.add('selected');
                }
                
                // Enable the Subscribe button
                const subscribeButton = parentCard.querySelector('.common-btn-primary[disabled]');
                if (subscribeButton) {
                    subscribeButton.removeAttribute('disabled');
                }
                
                // Close dropdown
                const selectItems = dropdown.querySelector('.select-items');
                selectItems.classList.remove('show');
                
                // Update aria-expanded
                const selectSelected = dropdown.querySelector('.select-selected');
                if (selectSelected) {
                    selectSelected.setAttribute('aria-expanded', 'false');
                }
            });
            
            // Add the new app to the top of the list for better visibility
            if (selectItemsContainer.firstChild) {
                selectItemsContainer.insertBefore(newAppItem, selectItemsContainer.firstChild);
            } else {
                selectItemsContainer.appendChild(newAppItem);
            }
        });
    }

    const apiCards = document.querySelectorAll(".api-card");
    apiCards.forEach(card => {
        const dropdown = card.querySelector(".custom-dropdown");

        if (dropdown) {
            // Custom select functionality
            const selectSelected = dropdown.querySelector(".select-selected");
            const selectItems = dropdown.querySelector(".select-items");
            const searchInput = dropdown.querySelector(".select-search-input");
            const selectItemsContainer = dropdown.querySelector(".select-items-container");
            const createAppOption = dropdown.querySelector(".create-app-option");
            
            // Select first non-subscribed app by default
            const selectFirstAvailableApp = () => {
                // Check if url query params has app ID
                let params = new URLSearchParams(window.location.search);
                let appId = "";
                let appFromParams = false;
                
                // Get application ID from URL parameters if available
                if (params.has('appID')) {
                    appId = params.get('appID');
                    appFromParams = true;
                }
                
                // If we have an app ID from params, try to find and select that app
                if (appFromParams && appId) {
                    const appItem = dropdown.querySelector(`.select-item[data-value="${appId}"]`);
                    if (appItem) {
                        const appName = appItem.getAttribute("data-app-name");
                        // Update hidden input with selected app ID
                        const hiddenField = document.getElementById(
                            dropdown.querySelector("[id^='selectedAppId-']").id
                        );
                        if (hiddenField) {
                            hiddenField.value = appId;
                        }
                        
                        // Update the display text
                        const selectedText = dropdown.querySelector(".selected-text");
                        if (selectedText) {
                            selectedText.textContent = appName;
                            selectedText.classList.add("selected");
                        }
                        
                        // Check if this app is already subscribed (disabled)
                        const subscribeButton = card.querySelector(".common-btn-primary[disabled]");
                        if (appItem.classList.contains("disabled")) {
                            // Keep the button disabled if the app is already subscribed
                            if (subscribeButton && !subscribeButton.disabled) {
                                subscribeButton.setAttribute("disabled", "disabled");
                            }
                        } else {
                            // Enable the Subscribe button if app is not already subscribed
                            if (subscribeButton) {
                                subscribeButton.removeAttribute("disabled");
                            }
                        }
                        return; // Exit early as we've handled the app selection from URL params
                    }
                }
                
                // Fall back to selecting the first available app if no app from params or app from params not found
                const firstAvailableApp = dropdown.querySelector(".select-item:not(.disabled)");
                if (firstAvailableApp) {
                    appId = firstAvailableApp.getAttribute("data-value");
                    const appName = firstAvailableApp.getAttribute("data-app-name");
                    // Update hidden input with selected app ID
                    const hiddenField = document.getElementById(
                        dropdown.querySelector("[id^='selectedAppId-']").id
                    );
                    if (hiddenField) {
                        hiddenField.value = appId;
                    }
                    
                    // Update the display text
                    const selectedText = dropdown.querySelector(".selected-text");
                    if (selectedText) {
                        selectedText.textContent = appName;
                        selectedText.classList.add("selected");
                    }
                    
                    // Enable the Subscribe button
                    const subscribeButton = card.querySelector(".common-btn-primary[disabled]");
                    if (subscribeButton) {
                        subscribeButton.removeAttribute("disabled");
                    }
                }
            };
            
            // Call this function when the page loads
            selectFirstAvailableApp();

            // Toggle dropdown when clicking on the selected item
            selectSelected.addEventListener("click", function (e) {
                e.stopPropagation();
                selectItems.classList.toggle("show");
                selectSelected.setAttribute("aria-expanded",
                    selectItems.classList.contains("show") ? "true" : "false");
                
                // Focus on search input when dropdown is opened
                if (selectItems.classList.contains("show") && searchInput) {
                    setTimeout(() => searchInput.focus(), 100);
                }
            });
            
            // Add search functionality
            if (searchInput) {
                searchInput.addEventListener("input", function(e) {
                    const searchValue = e.target.value.trim();
                    const searchValueLower = searchValue.toLowerCase();
                    const appItems = selectItemsContainer.querySelectorAll(".select-item");
                    const createAppContainer = dropdown.querySelector(".create-app-container");
                    const searchTermElement = dropdown.querySelector(".search-term");
                    let exactMatch = false;
                    
                    appItems.forEach(item => {
                        const appName = item.getAttribute("data-app-name").toLowerCase();
                        if (appName.includes(searchValueLower)) {
                            item.style.display = "flex";
                            if (appName === searchValueLower) {
                                exactMatch = true;
                            }
                        } else {
                            item.style.display = "none";
                        }
                    });
                    
                    // Update the search term in the create option and show/hide create option
                    if (searchValue && searchTermElement) {
                        searchTermElement.textContent = searchValue;
                        if (exactMatch || !searchValue) {
                            createAppContainer.style.display = "none";
                        } else {
                            createAppContainer.style.display = "block";
                        }
                    } else {
                        createAppContainer.style.display = "none";
                    }
                });
                
                // Prevent dropdown from closing when clicking in search input
                searchInput.addEventListener("click", function(e) {
                    e.stopPropagation();
                });
            }
            
            // Handle selection of application items
            const selectableItems = dropdown.querySelectorAll(".select-item:not(.disabled)");
            selectableItems.forEach(item => {
                item.addEventListener("click", function(e) {
                    e.stopPropagation();
                    
                    // Get application data
                    const appId = this.getAttribute("data-value");
                    const appName = this.getAttribute("data-app-name");
                    
                    // Update hidden input with selected app ID
                    const hiddenField = document.getElementById(
                        dropdown.querySelector("[id^='selectedAppId-']").id
                    );
                    if (hiddenField) {
                        hiddenField.value = appId;
                    }
                    
                    // Update the display text
                    const selectedText = dropdown.querySelector(".selected-text");
                    if (selectedText) {
                        selectedText.textContent = appName;
                        selectedText.classList.add("selected");
                    }
                    
                    // Enable the Subscribe button by removing the disabled attribute
                    const subscribeButton = card.querySelector(".common-btn-primary[disabled]");
                    if (subscribeButton) {
                        subscribeButton.removeAttribute("disabled");
                    }
                    
                    // Close dropdown
                    selectItems.classList.remove("show");
                    
                    // Update aria-expanded attribute
                    const combobox = dropdown.querySelector("[role='combobox']");
                    if (combobox) {
                        combobox.setAttribute("aria-expanded", "false");
                    }
                });
            });

            // Function to create application directly via API
            async function createApplicationDirectly(name, description = '') {
                try {
                    const response = await fetch('/devportal/applications', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                        },
                        body: JSON.stringify({
                            name,
                            description,
                            type: 'WEB',
                        }),
                    });

                    if (!response.ok) {
                        const errorData = await response.json();
                        throw new Error(errorData.message || `HTTP error! Status: ${response.status}`);
                    }

                    const responseData = await response.json();
                    
                    return responseData; // Return the full response to access applicationId
                } catch (error) {
                    console.error('Error creating application:', error);
                    throw error;
                }
            }

            // Handle create app option click
            if (createAppOption) {
                createAppOption.addEventListener("click", function(e) {
                    e.stopPropagation();
                    
                    const appName = searchInput.value.trim();
                    if (appName) {
                        // Show loading state
                        createAppOption.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> Creating...';
                        createAppOption.classList.add('disabled');
                        
                        // Call the API to create the application directly
                        createApplicationDirectly(appName)
                            .then(response => {
                                // Update hidden input with the new application ID
                                const hiddenField = document.getElementById(
                                    dropdown.querySelector("[id^='selectedAppId-']").id
                                );
                                if (hiddenField) {
                                    hiddenField.value = response.id;
                                }
                                
                                // Update the display text to show the new application
                                const selectedText = dropdown.querySelector(".selected-text");
                                if (selectedText) {
                                    selectedText.textContent = appName;
                                    selectedText.classList.add("selected");
                                }
                                
                                // Find the message overlay of the current card and show success message
                                const messageOverlay = card.querySelector('.message-overlay');
                                if (messageOverlay && typeof window.showApiMessage === 'function') {
                                    window.showApiMessage(
                                        messageOverlay, 
                                        `Application "${appName}" created successfully!`, 
                                        'success'
                                    );
                                }
                                
                                // Add the new app to all API card dropdowns
                                addAppToAllDropdowns(response.id, appName, 'api-card');
                                
                                // Close the dropdown
                                selectItems.classList.remove("show");
                                selectSelected.setAttribute("aria-expanded", "false");

                                const subscribeButton = card.querySelector(".common-btn-primary");
                                if (subscribeButton) {
                                    // Find and enable the subscribe button if it's disabled
                                    subscribeButton.removeAttribute("disabled");
                                }

                                // Reset the create app button
                                createAppOption.innerHTML = `Create application "<span class="search-term"></span>"`;
                                createAppOption.classList.remove('disabled');
                                
                                // Reset search field
                                if (searchInput) {
                                    searchInput.value = '';
                                }
                                
                                // Reset visibility of all items
                                const appItems = selectItemsContainer?.querySelectorAll(".select-item");
                                appItems?.forEach(item => {
                                    item.style.display = "flex";
                                });
                            })
                            .catch(error => {
                                // Reset the create app button on error
                                createAppOption.innerHTML = `Create application "<span class="search-term"></span>"`;
                                createAppOption.classList.remove('disabled');
                                
                                // Find the message overlay of the current card and show error message
                                const messageOverlay = card.querySelector('.message-overlay');
                                if (messageOverlay && typeof window.showApiMessage === 'function') {
                                    window.showApiMessage(
                                        messageOverlay, 
                                        `Failed to create application: ${error.message}`, 
                                        'error'
                                    );
                                }
                                
                                // Close the dropdown on error
                                selectItems.classList.remove("show");
                                selectSelected.setAttribute("aria-expanded", "false");
                                
                                // Reset search field
                                if (searchInput) {
                                    searchInput.value = '';
                                }
                                
                                // Reset visibility of all items
                                const appItems = selectItemsContainer?.querySelectorAll(".select-item");
                                appItems?.forEach(item => {
                                    item.style.display = "flex";
                                });
                                
                                // Hide create app container
                                const createAppContainer = dropdown.querySelector(".create-app-container");
                                if (createAppContainer) {
                                    createAppContainer.style.display = "none";
                                }
                            });
                    }
                });
            }

            // Close dropdown when clicking outside
            document.addEventListener("click", function () {
                selectItems.classList.remove("show");
                selectSelected.setAttribute("aria-expanded", "false");
                
                // Clear search input when closing dropdown
                if (searchInput) {
                    searchInput.value = '';
                    
                    // Reset visibility of all items
                    const appItems = selectItemsContainer?.querySelectorAll(".select-item");
                    appItems?.forEach(item => {
                        item.style.display = "flex";
                    });
                    
                    // Hide create app container
                    const createAppContainer = dropdown.querySelector(".create-app-container");
                    if (createAppContainer) {
                        createAppContainer.style.display = "none";
                    }
                }
            });
        }
    });

    const subscriptionCards = document.querySelectorAll(".subscription-card");
    subscriptionCards.forEach(card => {
        const dropdown = card.querySelector(".custom-dropdown");
        const subscribeBtn = card.querySelector(".common-btn-primary");

        if (dropdown && subscribeBtn) {
            // Custom select functionality
            const selectSelected = dropdown.querySelector(".select-selected");
            const selectItems = dropdown.querySelector(".select-items");
            const searchInput = dropdown.querySelector(".select-search-input");
            const selectItemsContainer = dropdown.querySelector(".select-items-container");
            const createAppOption = dropdown.querySelector(".create-app-option");

            // Select first non-subscribed app by default
            const selectFirstAvailableApp = () => {
                // Select the first available app
                const firstAvailableApp = dropdown.querySelector(".select-item:not(.disabled)");
                if (firstAvailableApp) {
                    appId = firstAvailableApp.getAttribute("data-value");
                    const appName = firstAvailableApp.getAttribute("data-app-name");
                    // Update hidden input with selected app ID

                    const hiddenInput = dropdown.querySelector("input[type='hidden']");
                    if (hiddenInput) {
                        hiddenInput.value = appId;
                    }
                    
                    // Update the display text
                    const selectedText = selectSelected.querySelector(".selected-text");
                    if (selectedText) {
                        selectedText.textContent = appName;
                        selectedText.classList.add("selected");
                    }
                    
                    // Check if this app is already subscribed (disabled)
                    const subscribeButton = card.querySelector(".common-btn-primary[disabled]");
                    if (subscribeButton) {
                        subscribeButton.removeAttribute("disabled");
                    }
                }
            };
            
            // Call this function when the page loads
            selectFirstAvailableApp();

            // Toggle dropdown when clicking on the selected item
            selectSelected.addEventListener("click", function (e) {
                e.stopPropagation();
                selectItems.classList.toggle("show");
                selectSelected.setAttribute("aria-expanded",
                    selectItems.classList.contains("show") ? "true" : "false");
                
                // Focus on search input when dropdown is opened
                if (selectItems.classList.contains("show") && searchInput) {
                    setTimeout(() => searchInput.focus(), 100);
                }
            });

            // Add search functionality
            if (searchInput) {
                searchInput.addEventListener("input", function(e) {
                    const searchValue = e.target.value.trim();
                    const searchValueLower = searchValue.toLowerCase();
                    const appItems = selectItemsContainer.querySelectorAll(".select-item");
                    const createAppContainer = dropdown.querySelector(".create-app-container");
                    const searchTermElement = dropdown.querySelector(".search-term");
                    let exactMatch = false;
                    
                    appItems.forEach(item => {
                        const appName = item.getAttribute("data-app-name").toLowerCase();
                        if (appName.includes(searchValueLower)) {
                            item.style.display = "flex";
                            if (appName === searchValueLower) {
                                exactMatch = true;
                            }
                        } else {
                            item.style.display = "none";
                        }
                    });
                    
                    // Update the search term in the create option and show/hide create option
                    if (searchValue && searchTermElement) {
                        searchTermElement.textContent = searchValue;
                        if (exactMatch || !searchValue) {
                            createAppContainer.style.display = "none";
                        } else {
                            createAppContainer.style.display = "block";
                        }
                    } else {
                        createAppContainer.style.display = "none";
                    }
                });
                
                // Prevent dropdown from closing when clicking in search input
                searchInput.addEventListener("click", function(e) {
                    e.stopPropagation();
                });
            }

            // Handle selection of application items
            const selectableItems = dropdown.querySelectorAll(".select-item:not(.disabled)");
            selectableItems.forEach(item => {
                item.addEventListener("click", function(e) {
                    e.stopPropagation();
                    
                    // Get application data
                    const appId = this.getAttribute("data-value");
                    const appName = this.getAttribute("data-app-name");
                    
                    // Update hidden input with selected app ID
                    const hiddenInput = dropdown.querySelector("input[type='hidden']");
                    if (hiddenInput) {
                        hiddenInput.value = appId;
                    }
                    
                    // Update the display text
                    const selectedText = selectSelected.querySelector(".selected-text");
                    if (selectedText) {
                        selectedText.textContent = appName;
                        selectedText.classList.add("selected");
                    }
                    
                    // Enable the Subscribe button by removing the disabled attribute
                    const subscribeButton = card.querySelector(".common-btn-primary[disabled]");
                    if (subscribeButton) {
                        subscribeButton.removeAttribute("disabled");
                    }
                    
                    // Close dropdown
                    selectItems.classList.remove("show");
                    
                    // Update aria-expanded attribute
                    selectSelected.setAttribute("aria-expanded", "false");
                });
            });

            // Function to create application directly via API
            async function createApplicationDirectly(name, description = '') {
                try {
                    const response = await fetch('/devportal/applications', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                        },
                        body: JSON.stringify({
                            name,
                            description,
                            type: 'WEB',
                        }),
                    });

                    if (!response.ok) {
                        const errorData = await response.json();
                        throw new Error(errorData.message || `HTTP error! Status: ${response.status}`);
                    }

                    const responseData = await response.json();
                    
                    return responseData; // Return the full response to access applicationId
                } catch (error) {
                    console.error('Error creating application:', error);
                    throw error;
                }
            }

            // Handle create app option click
            if (createAppOption) {
                createAppOption.addEventListener("click", function(e) {
                    e.stopPropagation();
                    
                    const appName = searchInput.value.trim();
                    if (appName) {
                        // Show loading state
                        createAppOption.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> Creating...';
                        createAppOption.classList.add('disabled');
                        
                        // Call the API to create the application directly
                        createApplicationDirectly(appName)
                            .then(response => {
                                // Update hidden input with the new application ID
                                const hiddenInput = dropdown.querySelector("input[type='hidden']");
                                if (hiddenInput) {
                                    hiddenInput.value = response.id;
                                }
                                
                                // Update the display text to show the new application
                                const selectedText = selectSelected.querySelector(".selected-text");
                                if (selectedText) {
                                    selectedText.textContent = appName;
                                    selectedText.classList.add("selected");
                                }

                                // Find the message overlay of the current card and show success message
                                const messageOverlay = card.querySelector('.message-overlay');
                                if (messageOverlay && typeof window.showApiMessage === 'function') {
                                    window.showApiMessage(
                                        messageOverlay, 
                                        `Application "${appName}" created successfully!`, 
                                        'success'
                                    );
                                }
                                
                                // Add the new app to all subscription card dropdowns
                                addAppToAllDropdowns(response.id, appName, 'subscription-card');
                                
                                // Close the dropdown
                                selectItems.classList.remove("show");
                                selectSelected.setAttribute("aria-expanded", "false");

                                const subscribeButton = card.querySelector(".common-btn-primary");
                                if (subscribeButton) {
                                    subscribeButton.removeAttribute("disabled");
                                }

                                // Reset the create app button
                                createAppOption.innerHTML = `Create application "<span class="search-term"></span>"`;
                                createAppOption.classList.remove('disabled');
                                
                                // Reset search field
                                if (searchInput) {
                                    searchInput.value = '';
                                }
                                
                                // Reset visibility of all items
                                const appItems = selectItemsContainer?.querySelectorAll(".select-item");
                                appItems?.forEach(item => {
                                    item.style.display = "flex";
                                });
                            })
                            .catch(error => {
                                // Reset the button on error
                                createAppOption.innerHTML = `Create application "<span class="search-term"></span>"`;
                                createAppOption.classList.remove('disabled');
                                
                                // Find the message overlay of the current card and show error message
                                const messageOverlay = card.querySelector('.message-overlay');
                                
                                if (messageOverlay && typeof window.showApiMessage === 'function') {
                                    window.showApiMessage(
                                        messageOverlay, 
                                        `Failed to create application: ${error.message}`, 
                                        'error'
                                    );
                                }
                                
                                // Close the dropdown on error
                                selectItems.classList.remove("show");
                                selectSelected.setAttribute("aria-expanded", "false");
                                
                                // Reset search field
                                if (searchInput) {
                                    searchInput.value = '';
                                }
                                
                                // Reset visibility of all items
                                const appItems = selectItemsContainer?.querySelectorAll(".select-item");
                                appItems?.forEach(item => {
                                    item.style.display = "flex";
                                });
                                
                                // Hide create app container
                                const createAppContainer = dropdown.querySelector(".create-app-container");
                                if (createAppContainer) {
                                    createAppContainer.style.display = "none";
                                }
                            });
                    }
                });
            }

            // Close dropdown when clicking outside
            document.addEventListener("click", function () {
                selectItems.classList.remove("show");
                selectSelected.setAttribute("aria-expanded", "false");
                
                // Clear search input when closing dropdown
                if (searchInput) {
                    searchInput.value = '';
                    
                    // Reset visibility of all items
                    const appItems = selectItemsContainer?.querySelectorAll(".select-item");
                    appItems?.forEach(item => {
                        item.style.display = "flex";
                    });
                    
                    // Hide create app container
                    const createAppContainer = dropdown.querySelector(".create-app-container");
                    if (createAppContainer) {
                        createAppContainer.style.display = "none";
                    }
                }
            });
        }
    });

    // Copy MCP Server Config JSON to clipboard
    window.copyServerConfig = async function(apiID) {
        const preBlock = document.getElementById(`server-config-${apiID}`);
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
            icon.classList.toggle('bi-chevron-down');
            icon.classList.toggle('bi-chevron-up');
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


function getOrgName() {

    const orgName = document.getElementById("organization-name").value;
    const url = new URL(window.location.origin + '/'+ orgName);
    window.location.href = url.toString();

}

