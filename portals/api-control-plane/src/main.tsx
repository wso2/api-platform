import React from 'react';
import ReactDOM from 'react-dom/client';

import { loadRuntimeConfigScripts } from './config/loadRuntimeConfigScripts';

const root = ReactDOM.createRoot(document.getElementById('root')!);

loadRuntimeConfigScripts()
  .catch((error) => {
    console.warn('Runtime config scripts could not be loaded.', error);
  })
  .finally(async () => {
    const { default: App } = await import('./App');

    root.render(
      <React.StrictMode>
        <App />
      </React.StrictMode>
    );
  });
