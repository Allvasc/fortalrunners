# FortalRunners — mobile (Expo)

```bash
npm install
npx expo start        # dev server; use um dev client (Expo Go não tem background location)
```

Ajuste a URL da API em `app.json → expo.extra.apiUrl`
(simulador iOS: `http://localhost:8090`; emulador Android: `http://10.0.2.2:8090`;
device físico: o IP da sua máquina).

## O que já tem (Fase 1)

- **Login / cadastro** (tokens no SecureStore, refresh silencioso).
- **Gravação de corrida** (`src/lib/recorder.ts`): `expo-location` + `expo-task-manager`
  com foreground service — grava com a tela apagada. Pontos vão para o AsyncStorage
  (canal task↔tela); distância/pace recalculados espelhando o `clean.go` do backend.
- **Tela ativa** (`Recording`): km, tempo, pace, contagem de pontos, prévia SVG do
  traçado (só a forma — o mapa real com tiles é Fase 2), pausar/retomar/concluir.
- **Upload** `POST /v1/runs` → **Resumo** com distância, pace, elevação, território
  ganho e status (poll enquanto o worker processa).
- **Home**: acumulado vitalício + últimas corridas.

Precisa de **dev client** (`npx expo run:android` / `run:ios` ou EAS Build) — o
background location não roda no Expo Go.

## Próximas fases

- Mapa: `@maplibre/maplibre-react-native` + tiles MapTiler/Protomaps (style JSON próprio).
- Fusão de sensores (acelerômetro/pedômetro/barômetro) para precisão e anti-fraude.
- 2FA no fluxo de login; conectar Strava; missões e selos; mapa de território ao vivo.
