import '@mantine/core/styles.css';
import '@mantine/notifications/styles.css';
import '@/styles.css';

import { createTheme, MantineProvider } from '@mantine/core';
import { Notifications } from '@mantine/notifications';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { HashRouter } from 'react-router';

import App from '@/App';

const theme = createTheme({
  primaryColor: 'blue',
  defaultRadius: 'md',
  fontFamily: 'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
  headings: {
    fontFamily: 'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
  },
});

const root = document.getElementById('root');

if (!root) {
  throw new Error('Root element was not found');
}

createRoot(root).render(
  <StrictMode>
    <MantineProvider defaultColorScheme="auto" theme={theme}>
      <Notifications position="bottom-right" />
      <HashRouter>
        <App />
      </HashRouter>
    </MantineProvider>
  </StrictMode>,
);
