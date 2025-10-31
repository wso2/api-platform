document.addEventListener('DOMContentLoaded', (event) => {
    const params = new URLSearchParams(window.location.search);
    const tags = params.get("tags");
    if (tags) {
        const tagArray = tags.split(',');
        tagArray.forEach(tag => {
            const element = document.getElementById(tag.toLowerCase());
            if (element) {
                element.checked = true;
            }
        });
    }
});

document.querySelectorAll('.form-check-input').forEach(checkbox => {
    checkbox.addEventListener('change', () => {
        let selectedTags = [];
        document.querySelectorAll('.form-check-input:checked').forEach(checkedCheckbox => {
            selectedTags.push(checkedCheckbox.getAttribute('data-tag'));
        });
        const tagsParam = selectedTags.join(',');
        window.location.href = `${window.location.pathname}?tags=${tagsParam}`;
    });
});

const clearAllButton = document.getElementById('clearAll');
if (clearAllButton) {
    clearAllButton.addEventListener('click', () => {
        document.querySelectorAll('.form-check-input:checked').forEach(checkbox => {
            checkbox.checked = false;
        });
        window.location.href = window.location.pathname; // Refresh URL without parameters
    });
}