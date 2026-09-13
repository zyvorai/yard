import { Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { getToken } from "./lib/api";
import Shell from "./components/Shell";
import Login from "./pages/Login";
import AcceptInvite from "./pages/AcceptInvite";
import ResetPassword from "./pages/ResetPassword";
import Overview from "./pages/Overview";
import Assets from "./pages/Assets";
import Sites from "./pages/Sites";
import MapPage from "./pages/MapPage";
import Telemetry from "./pages/Telemetry";
import WorkOrders from "./pages/WorkOrders";
import Incidents from "./pages/Incidents";
import Automations from "./pages/Automations";
import Integrations from "./pages/Integrations";
import Admin from "./pages/Admin";
import Diagnostics from "./pages/Diagnostics";
import Settings from "./pages/Settings";
import Onboarding from "./pages/Onboarding";

function Guard({ children }: { children: JSX.Element }) {
  if (!getToken()) return <Navigate to="/login" replace />;
  return children;
}

export default function App() {
  const loc = useLocation();
  const nav = useNavigate();
  if (loc.pathname === "/login") return <Login onIn={() => nav("/")} />;
  if (loc.pathname === "/accept-invite") return <AcceptInvite onIn={() => nav("/")} />;
  if (loc.pathname === "/reset-password") return <ResetPassword />;
  return (
    <Guard>
      <Shell>
        <Routes>
          <Route path="/" element={<Overview />} />
          <Route path="/assets" element={<Assets />} />
          <Route path="/sites" element={<Sites />} />
          <Route path="/map" element={<MapPage />} />
          <Route path="/telemetry" element={<Telemetry />} />
          <Route path="/work" element={<WorkOrders />} />
          <Route path="/incidents" element={<Incidents />} />
          <Route path="/automations" element={<Automations />} />
          <Route path="/integrations" element={<Integrations />} />
          <Route path="/admin" element={<Admin />} />
          <Route path="/diagnostics" element={<Diagnostics />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/onboarding" element={<Onboarding />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Shell>
    </Guard>
  );
}
