import { NavLink, Navigate, Route, Routes, useLocation } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type PublicUser } from "./lib/api";
import { useAuthed } from "./lib/useAuth";
import { Auth } from "./pages/Auth";
import { MapView } from "./pages/MapView";
import { Ranking } from "./pages/Ranking";
import { Challenges } from "./pages/Challenges";
import { Profile } from "./pages/Profile";

export function App() {
  const authed = useAuthed();
  if (!authed) return <Auth />;
  return <Portal />;
}

function Portal() {
  const qc = useQueryClient();
  const loc = useLocation();
  const me = useQuery<PublicUser>({ queryKey: ["me"], queryFn: api.me });

  return (
    <div className="app">
      <aside className="rail">
        <div className="brand">
          <span className="mark" aria-hidden />
          FortalRunners
        </div>
        <nav>
          <NavLink to="/" end>
            Mapa
          </NavLink>
          <NavLink to="/ranking">Ranking</NavLink>
          <NavLink to="/desafios">Desafios</NavLink>
          <NavLink to="/perfil">Perfil</NavLink>
        </nav>
        <div className="rail-foot">
          {me.data && (
            <>
              <div className="who">
                <strong>{me.data.username}</strong>
                <span className="muted small">{me.data.athlete_id}</span>
              </div>
              <button
                className="btn ghost sm"
                onClick={async () => {
                  await api.logout();
                  qc.clear();
                }}
              >
                Sair
              </button>
            </>
          )}
        </div>
      </aside>

      <main key={loc.pathname}>
        <Routes>
          <Route path="/" element={<MapView />} />
          <Route path="/ranking" element={<Ranking />} />
          <Route path="/desafios" element={<Challenges />} />
          <Route path="/perfil" element={<Profile />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  );
}
