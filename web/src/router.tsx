import { createBrowserRouter } from 'react-router';
import { AppLayout } from './components/AppLayout';
import { RequireAuth } from './components/RequireAuth';
import { LibraryPage } from './routes/LibraryPage';
import { LoginPage } from './routes/LoginPage';
import { NotFoundPage } from './routes/NotFoundPage';
import { RegisterPage } from './routes/RegisterPage';
import { RulesPage } from './routes/RulesPage';
import { SeriesDetailPage } from './routes/SeriesDetailPage';
import { SettingsPage } from './routes/SettingsPage';

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  { path: '/register', element: <RegisterPage /> },
  {
    element: <RequireAuth />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { path: '/', element: <LibraryPage /> },
          // NOT /series/:id — that path is the API's GET /series/{id}, which a
          // browser navigation (Lax cookie) would hit directly (ADR-0010 §4).
          { path: '/library/:id', element: <SeriesDetailPage /> },
          { path: '/rules', element: <RulesPage /> },
          { path: '/settings', element: <SettingsPage /> },
          // The server 200-serves index.html for every unknown GET, so the
          // client owns the 404 experience.
          { path: '*', element: <NotFoundPage /> },
        ],
      },
    ],
  },
]);
