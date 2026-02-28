import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Layout from './components/Layout';
import Status from './pages/Status';
import Logs from './pages/Logs';
import Rules from './pages/Rules';
import RuleSets from './pages/RuleSets';
import CacheRulePanel from './pages/CacheRulesPanel';
import BypassRulesPanel from './pages/BypassRulesPanel';
import CountryBlockPanel from './pages/CountryBlockPanel';
import RateLimitPanel from './pages/RateLimitPanel';

function App() {
  return (
    <BrowserRouter basename={import.meta.env.VITE_APP_BASE_PATH}>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Navigate to="/status" />} />
          <Route path="status" element={<Status />} />
          <Route path="logs" element={<Logs />} />
          <Route path="rules" element={<Rules />} />
          <Route path="rule-sets" element={<RuleSets />} />
          <Route path="bypass" element={<BypassRulesPanel />} />
          <Route path="country-block" element={<CountryBlockPanel />} />
          <Route path="rate-limit" element={<RateLimitPanel />} />
          <Route path="cache" element={<CacheRulePanel />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
