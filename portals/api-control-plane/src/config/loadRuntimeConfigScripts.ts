const CONFIG_SCRIPT_NAMES = [
  'api-platform.env.config.js',
  'api-platform.common.config.js',
];

// authMode is always emitted by the BFF's runtime config — its presence
// means the synchronous inline <script> tags in index.html already ran
// (they execute during HTML parsing, before this deferred module), so there
// is nothing left to fetch.
const hasRuntimeConfigLoaded = () =>
  Boolean(window.__RUNTIME_CONFIG__?.authMode);

const normalizeBasePath = (basePath: string) =>
  basePath === '/' ? '' : `/${basePath.replace(/^\/|\/$/g, '')}`;

const getScriptCandidates = (scriptName: string) => {
  const basePath = normalizeBasePath(
    import.meta.env.VITE_APP_BASE_PATH || import.meta.env.BASE_URL || '/'
  );

  return Array.from(
    new Set([
      basePath ? `${basePath}/${scriptName}` : `/${scriptName}`,
      `/${scriptName}`,
    ])
  );
};

const loadScript = (src: string) =>
  new Promise<void>((resolve, reject) => {
    const existingScript = document.querySelector<HTMLScriptElement>(
      `script[src="${src}"]`
    );

    // index.html inlines the config scripts (via %BASE_URL%...config.js). Those
    // are synchronous, non-deferred scripts, so they execute during HTML parsing
    // — before this deferred module entry runs. An existing tag has therefore
    // already loaded; re-attaching a `load` listener would never fire (the event
    // already happened), hanging the boot promise. Treat any pre-existing tag as
    // loaded; only genuinely-new scripts are appended and awaited below.
    if (existingScript) {
      resolve();
      return;
    }

    const script = existingScript ?? document.createElement('script');
    script.src = src;
    script.async = false;
    script.dataset.runtimeConfig = 'true';

    script.addEventListener('load', () => {
      script.dataset.loaded = 'true';
      resolve();
    });
    script.addEventListener('error', () => reject(new Error(src)));

    if (!existingScript) {
      document.head.appendChild(script);
    }
  });

const loadFirstAvailableScript = async (scriptName: string) => {
  const failedSources: string[] = [];

  for (const src of getScriptCandidates(scriptName)) {
    try {
      await loadScript(src);
      return;
    } catch {
      failedSources.push(src);
    }
  }

  throw new Error(`Unable to load runtime config script: ${failedSources.join(', ')}`);
};

export const loadRuntimeConfigScripts = async () => {
  if (hasRuntimeConfigLoaded()) return;

  for (const scriptName of CONFIG_SCRIPT_NAMES) {
    await loadFirstAvailableScript(scriptName);
  }
};
