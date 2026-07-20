import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './auth/AuthContext.jsx';
import { ToastProvider } from './components/Toast.jsx';
import { Spinner } from './components/ui.jsx';
import Layout from './components/Layout.jsx';
import Login from './pages/Login.jsx';
import Products from './pages/Products.jsx';
import ProductInventory from './pages/ProductInventory.jsx';
import Telegram from './pages/Telegram.jsx';
import Orders from './pages/Orders.jsx';
import Deliveries from './pages/Deliveries.jsx';
import Settings from './pages/Settings.jsx';

function Protected({ children }) {
  const { token, loading } = useAuth();
  if (loading) return <Spinner label="Loading…" />;
  if (!token) return <Navigate to="/login" replace />;
  return children;
}

function AppRoutes() {
  const { token } = useAuth();
  return (
    <Routes>
      <Route path="/login" element={token ? <Navigate to="/products" replace /> : <Login />} />
      <Route
        element={
          <Protected>
            <Layout />
          </Protected>
        }
      >
        <Route path="/products" element={<Products />} />
        <Route path="/products/:id/inventory" element={<ProductInventory />} />
        <Route path="/telegram" element={<Telegram />} />
        <Route path="/orders" element={<Orders />} />
        <Route path="/deliveries" element={<Deliveries />} />
        <Route path="/settings" element={<Settings />} />
      </Route>
      <Route path="*" element={<Navigate to="/products" replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <ToastProvider>
          <AppRoutes />
        </ToastProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}
