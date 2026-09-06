# FortalRunners

App gamificado de conquista de território para corredores de Fortaleza.
Backend em **Go** (monólito modular + worker), web em **React**, mobile em **React Native (Expo)**.

## Documentação

| Documento | Onde |
|---|---|
| Plano de produto | `docs/plano.html` · [online](https://claude.ai/code/artifact/f171dbf5-8933-40a3-98ff-32f169ff869b) |
| Banco & Integrações | `docs/banco-e-integracoes.html` · [online](https://claude.ai/code/artifact/8bd97373-4a1d-4fa3-8144-4eefbfdeb34b) |
| Manual de Design | `docs/manual-de-design.html` · [online](https://claude.ai/code/artifact/e662d35e-b65a-4f5c-a0ad-d2480c77f2ac) |
| Políticas & Compliance | `docs/politicas.html` · [online](https://claude.ai/code/artifact/72fd46a4-ad3d-4e74-9045-8c463754c50d) |
| Mockups de tela | `docs/mockups/` · [online](https://claude.ai/code/artifact/d65150fe-12cf-4e14-ac58-49cbd29abb4e) |

## Estrutura do monorepo

```
backend/   Go — API (cmd/api) + worker de território (cmd/worker), módulos por domínio
web/       React + Vite + TypeScript — portal do corredor e painel de admin
mobile/    Expo (React Native) — app do corredor
infra/     docker-compose para dev local (Postgres+PostGIS, Redis, NATS)
docs/      documentação (HTML)
```

## Pré-requisitos

- Go 1.26+
- Node 20+ (recomendado 22/24)
- Docker + Docker Compose
- (mobile) Expo CLI: `npm i -g expo` — opcional, `npx` funciona

## Subir o ambiente local

```bash
# 1. serviços de infra (Postgres+PostGIS, Redis, NATS)
make infra-up

# 2. backend — migrações + API
cd backend
cp .env.example .env
make migrate
make run          # API em http://localhost:8090  (GET /healthz)

# em outro terminal: worker de território
make run-worker

# 3. web
cd ../web
cp .env.example .env
npm install
npm run dev       # http://localhost:5173

# 4. mobile
cd ../mobile
npm install
npx expo start
```

## Roadmap

Ver `docs/plano.html` § 16.

- **Fase 0 — Fundação** ✅ monorepo, auth (e-mail + JWT rotativo), esquema de banco, health, CI.
- **Fase 1 — MVP jogável** 🚧 em andamento:
  - ✅ ingestão de corrida (`POST /v1/runs`), limpeza de GPS, `runs` + `run_tracks`
  - ✅ pipeline de território no worker: detecção de laço → polígono (PostGIS) → gate de zona de risco → gravação; `GET /v1/territories` (GeoJSON)
  - ✅ gestão de tênis (`/v1/shoes`), km/passos/horas/custo-por-km/vida-útil, corrida amarrada ao par
  - ✅ rollup pós-corrida: `shoe_stats`, `lifetime_stats` (com streak), `personal_records`
  - ✅ ranking **all-time por área coberta** (`/v1/leaderboards/global`, `/neighborhood/:id`) — território é
    permanente e pessoal, ninguém apaga ou disputa o do outro
  - ✅ **desafios recorrentes** (semanal / quinzenal / mensal): `/v1/challenges`,
    `/v1/challenges/:slug/leaderboard`; `cmd/scheduler` abre e fecha os períodos e concede badges
  - ✅ `/v1/me/lifetime`, `/v1/me/records`
  - ✅ **2FA (TOTP, RFC 6238)**: `/v1/auth/mfa/{setup,activate,disable}` + desafio no login
    (`/v1/auth/mfa/verify`); segredo cifrado em repouso (AES-256-GCM, `MFA_ENC_KEY`),
    10 códigos de recuperação de uso único, claim `mfa` no access token +
    middleware `RequireMFA` para rotas privilegiadas
  - ✅ **métricas de precisão** (`internal/run/metrics.go`): splits por km, elevação
    suavizada (histerese barométrica), cadência e FC dos streams de sensor, pace
    ajustado ao aclive (GAP, modelo de custo de Minetti); `GET /v1/runs/:id/metrics`
  - ✅ **mapa de calor pessoal** (`internal/heatmap`): `heat_agg` (grade ~50 m como
    stand-in de H3), refresh no `cmd/scheduler`, `GET /v1/heatmap?scope=me` (GeoJSON);
    amigos/cidade + k-anonimato = Fase 2/3
  - ✅ **painel de admin** (`internal/admin`, `/v1/admin/*`): role admin/moderator + 2FA
    verificado (conta privilegiada sem TOTP não entra); `audit_log` append-only em toda
    mutação; `game_config` runtime (flags/parâmetros — `risk_zone_blocking` etc.); rotas
    de usuários, corridas sinalizadas, risk-zones CRUD, config, auditoria. UI web em `/admin`
  - ✅ **portal web** (`web/`): rota + shell, mapa MapLibre com território + camada
    de calor ligável, histórico com splits/GAP/cadência, ranking all-time, desafios,
    perfil com stats/recordes e gestão de 2FA, painel de admin (role-gated)
  - ✅ **login social** (`internal/auth/oauth.go`): Google (OIDC) + Apple Sign In
    (client-secret JWT ES256 do .p8), state assinado anti-CSRF, adaptador por provedor;
    `GET /v1/auth/oauth/:provider(/callback)`, `?mode=link` para vincular à conta logada;
    provedor sem config → 501
  - ✅ **integração Strava** (`internal/integration`): OAuth connect, import de atividades
    (`data_source=import`, mesmo pipeline; dedup por `runs.import_ref` + janela de tempo),
    webhook idempotente (`webhook_events`), refresh de token, jobs (`import_jobs`) no
    `cmd/scheduler`; adaptador por provedor (Garmin/Fitbit/Polar depois). `crypto.Box`
    (AES-256-GCM) para os tokens
  - ✅ **app mobile** (`mobile/`): gravação de corrida (`expo-location` + `expo-task-manager`,
    foreground service — grava com tela apagada), cadência pelo pedômetro (`expo-sensors`),
    tela ativa (km/tempo/pace/prévia SVG), upload `POST /v1/runs` → resumo com território,
    **mapa MapLibre Native** com o território, 2FA no login. Precisa de dev client
  - ✅ **marcos históricos & selos** (`internal/landmark`): 15 cartões-postais de Fortaleza seeded,
    check-in por proximidade GPS e auto-checkin no pipeline de corrida com concessão de badges e progresso de coleções; UI no portal web (`/marcos`) e mobile (`Landmarks`)
  - ✅ **rotas comunitárias** (`internal/route`): percursos curados em Fortaleza (Beira-Mar 5k, Cocó 7k),
    avaliações com notas 1-5 estrelas e comentários; UI web (`/rotas`) e mobile (`Routes`)
  - ✅ **pontos de apoio urbanos** (`internal/poi`): bebedouros, banheiros públicos e postos de apoio em Fortaleza em GeoJSON (`GET /v1/amenities`), integrados como marcadores no mapa
  - ⬜ Health Connect / Apple Health (ponte on-device), style JSON próprio do mapa
    (chave MapTiler/Protomaps), fusão de sensores para precisão, cobertura H3 (%)

## Convenções

- Commits: [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `chore:`…).
- Backend: `golangci-lint`, `gofmt`/`goimports`, queries via `sqlc`, migrações reversíveis (`goose`).
- Segurança e arquitetura: ver `docs/plano.html` §§ 8, 14, 17, 18.
