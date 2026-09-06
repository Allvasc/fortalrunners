# Gerar o APK do FortalRunners (Android)

O app usa módulos nativos (GPS em background, câmera, MapLibre, SecureStore),
então **não roda no Expo Go** — precisa de um build próprio. O caminho mais
simples é o **EAS Build** (na nuvem da Expo, plano grátis).

## Pré-requisitos (uma vez)

1. Conta Expo grátis: https://expo.dev/signup
2. Node 20+ instalado
3. Instalar a CLI:
   ```bash
   npm install -g eas-cli
   ```

## Passo a passo

Tudo dentro de `mobile/`:

```bash
cd mobile
npm install

# 1. logar na sua conta Expo
eas login

# 2. linkar o projeto à sua conta (cria o projectId em app.json > extra.eas)
eas init

# 3. build do APK (perfil "preview" já configurado no eas.json)
eas build -p android --profile preview
```

A CLI vai:
- perguntar se pode **gerar um keystore** para você → responda **sim** (a Expo
  guarda; para publicar na Play Store depois é o mesmo keystore)
- subir o código e buildar na nuvem (~10–20 min na fila grátis)
- no fim, imprime uma **URL**. Abra no navegador → botão **Download** → `.apk`

## Instalar no celular

1. Copie o `.apk` para o telefone (cabo, Drive, Telegram pra si mesmo…)
2. Toque no arquivo → o Android pede pra permitir "instalar apps desconhecidos"
   para o app que está abrindo (Arquivos/Chrome) → permitir
3. Instalar → abrir

Na primeira tela crie uma conta ou use uma que você já registrou pelo portal web.

## O que o APK já aponta

O `eas.json` fixa `EXPO_PUBLIC_API_URL=https://fortalrunners-api.onrender.com`,
então o app fala direto com a API de produção. Nada a configurar.

> A API está no Render free: se ninguém usou nos últimos ~15 min ela "dorme" e a
> primeira requisição demora ~50s. Depois normaliza.

## Se o build falhar

- **Erro de versão de pacote Expo**: rode `npx expo install --check` e commite o
  `package.json` atualizado.
- **Erro nativo ligado à "new architecture"**: em `app.json` troque
  `"newArchEnabled": true` por `false` e rode de novo.
- O log completo fica na página do build no site da Expo.

## Alternativa: build local (sem conta Expo)

Precisa de Android Studio + JDK 17 + variável `ANDROID_HOME`:

```bash
cd mobile
npm install
npx expo prebuild -p android --clean
cd android
./gradlew assembleRelease
# APK em android/app/build/outputs/apk/release/app-release.apk
```

Esse APK usa um keystore de debug — serve pra testar, não pra publicar.

## Rodar em modo dev (iterar rápido, sem rebuildar a cada mudança)

```bash
npx expo install expo-dev-client
```

Adicione um perfil `development` no `eas.json`:

```json
"development": {
  "developmentClient": true,
  "distribution": "internal",
  "android": { "buildType": "apk" },
  "env": { "EXPO_PUBLIC_API_URL": "https://fortalrunners-api.onrender.com" }
}
```

```bash
eas build -p android --profile development   # 1 vez — instala esse APK
npx expo start --dev-client                   # recarrega só o JS
```
