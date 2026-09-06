import { NavLink, Navigate, Route, Routes, useLocation } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, consumeOAuthFragment, type PublicUser } from "./lib/api";
import { useAuthed } from "./lib/useAuth";
import { Auth } from "./pages/Auth";
import { ResetPassword } from "./pages/ResetPassword";
import { MapView } from "./pages/MapView";
import { History } from "./pages/History";
import { Evolution } from "./pages/Evolution";
import { Ranking } from "./pages/Ranking";
import { Challenges } from "./pages/Challenges";
import { Profile } from "./pages/Profile";
import { Admin } from "./pages/Admin";
import { LandmarksPage } from "./pages/Landmarks";
import { RoutesPage } from "./pages/Routes";
import { SocialPage } from "./pages/Social";
import { ClubsPage } from "./pages/Clubs";
import { ShoesPage } from "./pages/Shoes";

import { EventsPage } from "./pages/Events";
import { AICoachPage } from "./pages/AICoach";
import { Organizer } from "./pages/Organizer";

// Consome os tokens do callback OAuth antes de decidir a rota.
if (window.location.pathname === "/auth/callback") {
  consumeOAuthFragment();
}

export function App() {
  const authed = useAuthed();
  if (window.location.pathname === "/redefinir-senha") return <ResetPassword />;
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
          <NavLink to="/historico">Histórico</NavLink>
          <NavLink to="/evolucao">Evolução</NavLink>
          <NavLink to="/ranking">Ranking</NavLink>
          <NavLink to="/desafios">Desafios</NavLink>
          <NavLink to="/marcos">Marcos & Selos</NavLink>
          <NavLink to="/rotas">Rotas</NavLink>
          <NavLink to="/social">Social</NavLink>
          <NavLink to="/clubes">Clubes</NavLink>
          <NavLink to="/tenis">Tênis</NavLink>
          <NavLink to="/eventos">Eventos & QR</NavLink>
          <NavLink to="/coach">Coach IA</NavLink>
          <NavLink to="/perfil">Perfil</NavLink>
          {me.data && (me.data.role === "organizer" || me.data.role === "admin") && (
            <NavLink to="/organizador">Organizador</NavLink>
          )}
          {me.data && (me.data.role === "admin" || me.data.role === "moderator") && (
            <NavLink to="/admin">Admin</NavLink>
          )}
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
          <Route path="/historico" element={<History />} />
          <Route path="/evolucao" element={<Evolution />} />
          <Route path="/ranking" element={<Ranking />} />
          <Route path="/desafios" element={<Challenges />} />
          <Route path="/marcos" element={<LandmarksPage />} />
          <Route path="/rotas" element={<RoutesPage />} />
          <Route path="/social" element={<SocialPage />} />
          <Route path="/clubes" element={<ClubsPage />} />
          <Route path="/tenis" element={<ShoesPage />} />
          <Route path="/eventos" element={<EventsPage />} />
          <Route path="/coach" element={<AICoachPage />} />
          <Route path="/perfil" element={<Profile />} />
          <Route path="/organizador" element={<Organizer />} />
          <Route path="/admin" element={<Admin />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  );
}
