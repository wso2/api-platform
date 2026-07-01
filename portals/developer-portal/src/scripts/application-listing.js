// Search functionality for the application listing page

document.addEventListener('DOMContentLoaded', () => {
    const queryInput = document.getElementById('query');
    const applicationsContainer = document.getElementById(
        'applicationCardsContainer'
    );
    const allCards = Array.from(applicationsContainer.children);
    queryInput.addEventListener('input', () => {
        const query = queryInput.value.trim().toLowerCase();
        if (!query) {
            applicationsContainer.innerHTML = '';
            allCards.forEach((card) => {
                applicationsContainer.appendChild(card);
            });
            return;
        }
        const filteredCards = allCards.filter((card) => {
            const appDisplayName = card.getAttribute('data-display-name').toLowerCase();
            return appDisplayName.includes(query);
        });
        applicationsContainer.innerHTML = '';
        filteredCards.forEach((card) => {
            applicationsContainer.appendChild(card);
        });
    });
});
