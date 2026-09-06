# FortalRunners — mobile (Expo)

```bash
npm install
npx expo start        # abre o dev server; use o app Expo Go ou um dev client
```

Ajuste a URL da API em `app.json → expo.extra.apiUrl`
(no simulador iOS use `http://localhost:8080`; no Android emulador, `http://10.0.2.2:8080`;
em device físico, o IP da sua máquina).

## Próximas fases

- Mapa: `@maplibre/maplibre-react-native` + tiles MapTiler/Protomaps.
- Gravação em background: `react-native-background-geolocation` (precisa de dev client / prebuild).
- Sensores para precisão e anti-fraude: acelerômetro, pedômetro, barômetro.
