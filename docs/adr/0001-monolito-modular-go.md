# ADR 0001 — Monólito modular em Go, com worker separado

- **Status:** aceito
- **Data:** 2026-09-05

## Contexto

O FortalRunners tem ~8 domínios (identidade, corrida/território, gamificação,
social, eventos/QR, pagamentos, IA, segurança). O processamento de território é
pesado (map-matching, geometria PostGIS, H3, anti-fraude) e não pode ficar no
caminho da requisição do cliente.

## Decisão

Um **monólito modular** em Go: um único binário `api` com pacotes por domínio em
`internal/`, dependências apontando para dentro (interface → adaptador), e um
segundo binário `worker` que consome a fila e roda o pipeline de território.

Só o `worker` é processo separado. Um módulo só vira serviço próprio quando tiver
escala ou cadência de deploy comprovadamente distintas.

## Consequências

- Deploy e testes simples; transações locais no Postgres.
- A fronteira `api ↔ worker` já está desenhada — extrair mais serviços depois é
  incremental.
- Disciplina necessária: nada de `interface` acessando o banco direto, nada de
  regra de negócio chamando SDK de terceiro.

Ver `docs/plano.html` §§ 8, 18 e `docs/banco-e-integracoes.html` § 14.
