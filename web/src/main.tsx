import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '../../design/tokens.css';
import './styles/app.css';
import { App } from './App';

const rootEl = document.getElementById('root');
if (!rootEl) throw new Error('web: #root element missing');
createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
