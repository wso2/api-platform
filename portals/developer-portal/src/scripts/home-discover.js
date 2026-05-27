(function () {
    const dataEl = document.getElementById('homeDiscoverData');
    if (!dataEl) return;
    const { baseUrl } = JSON.parse(dataEl.textContent);
    const llmsUrl = window.location.origin + baseUrl + '/llms.txt';

    const lines = [
        'Please explore this developer portal by reading the API catalog index at:',
        llmsUrl,
        '',
        'Once you have read that document, please answer the following:',
        '',
        '1. What APIs are available in this portal? List each API with a brief description.',
        '2. What API workflows are available? Describe what each workflow does.',
        '3. What MCP Servers are available and what tools do they expose?',

        'After answering, ask for a use case suggest the most relevant API or workflow based on that.'
    ];
    const prompt = lines.join('\n');

    const preEl = document.getElementById('discoverPromptText');
    if (preEl) preEl.textContent = prompt;

    const downloadBtn = document.getElementById('discoverBtnDownloadPrompt');
    if (downloadBtn) {
        downloadBtn.addEventListener('click', function () {
            const blob = new Blob([prompt], { type: 'text/plain' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'discover-prompt.txt';
            a.click();
            URL.revokeObjectURL(url);
        });
    }

    const copyBtn = document.getElementById('discoverBtnCopyPrompt');
    if (copyBtn) {
        copyBtn.addEventListener('click', function () {
            navigator.clipboard.writeText(prompt).then(function () {
                const icon = copyBtn.querySelector('i');
                icon.classList.replace('bi-copy', 'bi-check2');
                setTimeout(function () { icon.classList.replace('bi-check2', 'bi-copy'); }, 1800);
            });
        });
    }

    const runBtn = document.getElementById('discoverBtnRunClaude');
    if (runBtn) {
        runBtn.addEventListener('click', function () {
            const cacheBustedUrl = llmsUrl + '?request-id=' + Math.floor(Math.random() * 1e10);
            const runPrompt = lines.map(line => line === llmsUrl ? cacheBustedUrl : line).join('\n');
            window.open('https://claude.ai/new?q=' + encodeURIComponent(runPrompt), '_blank');
        });
    }
})();
