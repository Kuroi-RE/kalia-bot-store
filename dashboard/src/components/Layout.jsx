import { NavLink, Outlet } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext.jsx';

const nav = [
  { to: '/products', label: '📦 Products' },
  { to: '/telegram', label: '💬 Telegram' },
  { to: '/orders', label: '🧾 Orders' },
  { to: '/confirmations', label: '✅ Confirm Pay' },
  { to: '/deliveries', label: '🚚 Deliveries' },
  { to: '/settings', label: '⚙️ Settings' },
];

export default function Layout() {
  const { admin, logout } = useAuth();

  return (
    <div className="min-h-screen">
      <header className="border-b-2 border-ink bg-brand">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-3">
          <div className="text-xl font-bold tracking-tight">
            KALIA<span className="bg-ink px-1 text-brand">STORE</span> ADMIN
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden text-sm font-bold sm:inline">👤 {admin?.username}</span>
            <button className="nb-btn nb-btn-danger px-3 py-1" onClick={logout}>
              Logout
            </button>
          </div>
        </div>
      </header>

      <div className="mx-auto flex max-w-6xl flex-col gap-4 p-4 md:flex-row">
        <aside className="md:w-52 shrink-0">
          <nav className="nb-card sticky top-4 flex flex-col gap-2 bg-white p-3">
            {nav.map((n) => (
              <NavLink
                key={n.to}
                to={n.to}
                className={({ isActive }) =>
                  `nb-btn justify-start ${isActive ? 'nb-btn-accent' : 'nb-btn-ghost'}`
                }
              >
                {n.label}
              </NavLink>
            ))}
          </nav>
        </aside>

        <main className="min-w-0 flex-1">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
