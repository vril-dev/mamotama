import { Link, Outlet, useLocation } from 'react-router-dom';

export default function Layout() {
  const { pathname } = useLocation();

  return (
    <div className="flex h-screen">
      <aside className="w-64 bg-gray-800 text-white p-4 space-y-4">
        <h2 className="text-xl font-bold">Mamotama Admin</h2>
        <nav className="flex flex-col space-y-2">
          <Link to="/status" className={pathname === '/status' ? 'font-bold' : ''}>Status</Link>
          <Link to="/logs" className={pathname === '/logs' ? 'font-bold' : ''}>Logs</Link>
          <Link to="/rules" className={pathname === '/rules' ? 'font-bold' : ''}>Rules</Link>
          <Link to="/bypass" className={pathname === '/bypass' ? 'font-bold' : ''}>Bypass</Link>
          <Link to="/cache" className={pathname === '/cache' ? 'font-bold' : ''}>Cache</Link>
          <Link to="/settings" className={pathname === '/settings' ? 'font-bold' : ''}>Settings</Link>
        </nav>
      </aside>
      <main className="flex-1 p-6 bg-gray-100 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  );
}
