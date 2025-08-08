import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Layout from './components/Layout';
import Status from './pages/Status';
import Logs from './pages/Logs';
import Rules from './pages/Rules';
import Settings from './pages/Settings';
import BypassEditor from './pages/BypassEditor';

function App() {
  return (
    <BrowserRouter basename={import.meta.env.VITE_APP_BASE_PATH}>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Navigate to="/status" />} />
          <Route path="status" element={<Status />} />
          <Route path="logs" element={<Logs />} />
          <Route path="rules" element={<Rules />} />
          <Route path="bypass" element={<BypassEditor />} />
          <Route path="settings" element={<Settings />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
