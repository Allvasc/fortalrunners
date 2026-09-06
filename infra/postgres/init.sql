-- Extensões usadas pelo FortalRunners. Roda uma vez, na criação do container.
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
-- h3-pg é opcional (indexação H3 também pode ser feita na aplicação via uber/h3-go).
-- A imagem postgis padrão não traz h3; habilite se usar uma imagem com a extensão:
-- CREATE EXTENSION IF NOT EXISTS h3;
