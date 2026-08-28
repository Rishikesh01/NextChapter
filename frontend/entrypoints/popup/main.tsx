import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '../../assets/tokens.css';
import '../../assets/ui.css';
import './style.css';
import { App } from './App';

const rootEl = document.getElementById('root');
if (!rootEl) throw new Error('popup: #root element missing');
createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
