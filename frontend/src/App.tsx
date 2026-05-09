import { lazy, Suspense, useEffect } from 'react';
import {
  HashRouter,
  Navigate,
  Outlet,
  Route,
  Routes,
} from 'react-router-dom';

import TitleBar from './components/TitleBar';
import Sidebar from './components/Sidebar';
import Toaster from './components/Toaster';
import ErrorBoundary from './components/ErrorBoundary';
import LoginBanner from './components/LoginBanner';
import ConfirmDialog from './components/ConfirmDialog';

// Pages are lazy-loaded so first paint only pays for the dashboard bundle.
// Settings + History are heavy (chart, filters, drawer) and can wait.
const DashboardPage = lazy(() => import('./pages/DashboardPage'));
const QueuePage = lazy(() => import('./pages/QueuePage'));
const HistoryPage = lazy(() => import('./pages/HistoryPage'));
const SettingsPage = lazy(() => import('./pages/SettingsPage'));

import { installEventBridge } from './store/eventBridge';
import { useAppStore } from './store/appStore';
import {
  EnsureBrowser,
  GetQueueState,
  GetBrowserStatus,
} from '../wailsjs/go/main/App';

function PageFallback() {
  return (
    <div className="flex flex-1 items-center justify-center text-sm text-subtle">
      Đang tải…
    </div>
  );
}

function RootLayout() {
  const setBrowser = useAppStore((s) => s.setBrowser);
  const setQueueState = useAppStore((s) => s.setQueueState);

  useEffect(() => {
    installEventBridge();
    GetBrowserStatus().then((s) => setBrowser(s as any)).catch(() => {});
    GetQueueState().then((s) => setQueueState(s as any)).catch(() => {});
    EnsureBrowser().catch(() => {
      // Failure surfaces via browserStatus event; ignore here.
    });
  }, [setBrowser, setQueueState]);

  return (
    <div className="flex h-screen flex-col surface-app">
      <TitleBar />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar />
        <main className="flex flex-1 flex-col overflow-auto">
          <BannerSlot />
          <Suspense fallback={<PageFallback />}>
            <Outlet />
          </Suspense>
        </main>
      </div>
      <Toaster />
      <ConfirmDialog />
    </div>
  );
}

function BannerSlot() {
  const status = useAppStore((s) => s.browser?.status ?? 'disconnected');
  if (status === 'ready') return null;
  return (
    <div className="px-6 pt-4">
      <LoginBanner />
    </div>
  );
}

export default function App() {
  return (
    <ErrorBoundary>
      <HashRouter>
        <Routes>
          <Route element={<RootLayout />}>
            <Route index element={<Navigate to="/dashboard" replace />} />
            <Route path="dashboard" element={<DashboardPage />} />
            <Route path="queue" element={<QueuePage />} />
            <Route path="history" element={<HistoryPage />} />
            <Route path="settings" element={<SettingsPage />} />
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Route>
        </Routes>
      </HashRouter>
    </ErrorBoundary>
  );
}
