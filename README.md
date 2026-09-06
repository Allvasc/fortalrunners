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
  - ✅ ranking (`/v1/leaderboards/global` e `/neighborhood/:id`), `/v1/me/lifetime`, `/v1/me/records`
  - ⬜ métricas de precisão (splits, cadência), mapa de calor, fechamento de ciclo semanal + troféus,
    integrações (Strava), 2FA, portal web (mapa MapLibre), painel de admin

## Convenções

- Commits: [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `chore:`…).
- Backend: `golangci-lint`, `gofmt`/`goimports`, queries via `sqlc`, migrações reversíveis (`goose`).
- Segurança e arquitetura: ver `docs/plano.html` §§ 8, 14, 17, 18.
