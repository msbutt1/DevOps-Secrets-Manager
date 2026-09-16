import { Toaster } from '@/components/ui/toaster';
import { Toaster as Sonner } from '@/components/ui/sonner';
import { TooltipProvider } from '@/components/ui/tooltip';
import { QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider } from '@/contexts/AuthContext';
import { OrganizationProvider } from '@/contexts/OrganizationContext';
import { ProtectedRoute } from '@/components/ProtectedRoute';
import { queryClient } from '@/lib/query-client';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import VerifyEmailPage from './pages/VerifyEmailPage';
import InvitePage from './pages/InvitePage';
import ForgotPasswordPage from './pages/ForgotPasswordPage';
import ResetPasswordPage from './pages/ResetPasswordPage';
import LegalPage from './pages/LegalPage';
import DashboardPage from './pages/DashboardPage';
import VaultsListPage from './pages/VaultsListPage';
import VaultPage from './pages/VaultPage';
import AccessPage from './pages/AccessPage';
import AuditPage from './pages/AuditPage';
import SettingsPage from './pages/SettingsPage';
import OrganizationPage from './pages/OrganizationPage';
import NotFound from './pages/NotFound';

const App = () => (
  <QueryClientProvider client={queryClient}>
    <TooltipProvider>
      <AuthProvider>
        <OrganizationProvider>
          <Toaster />
          <Sonner />
          <BrowserRouter>
            <Routes>
              <Route path="/login" element={<LoginPage />} />
              <Route path="/register" element={<RegisterPage />} />
              <Route path="/verify-email" element={<VerifyEmailPage />} />
              <Route path="/invite" element={<InvitePage />} />
              <Route path="/forgot-password" element={<ForgotPasswordPage />} />
              <Route path="/reset-password" element={<ResetPasswordPage />} />
              {/* One page, reachable under the names people look for */}
              <Route path="/legal" element={<LegalPage />} />
              <Route path="/privacy" element={<LegalPage />} />
              <Route path="/terms" element={<LegalPage />} />
              <Route
                path="/"
                element={
                  <ProtectedRoute>
                    <DashboardPage />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/vaults"
                element={
                  <ProtectedRoute>
                    <VaultsListPage />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/vaults/:id"
                element={
                  <ProtectedRoute>
                    <VaultPage />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/vaults/:id/access"
                element={
                  <ProtectedRoute>
                    <AccessPage />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/audit"
                element={
                  <ProtectedRoute>
                    <AuditPage />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/organization"
                element={
                  <ProtectedRoute>
                    <OrganizationPage />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/settings"
                element={
                  <ProtectedRoute>
                    <SettingsPage />
                  </ProtectedRoute>
                }
              />
              <Route path="*" element={<NotFound />} />
            </Routes>
          </BrowserRouter>
        </OrganizationProvider>
      </AuthProvider>
    </TooltipProvider>
  </QueryClientProvider>
);

export default App;
