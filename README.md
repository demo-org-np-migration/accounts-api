# accounts-api

Servicio de lectura de cuentas. Lee la base del ledger directamente por performance (ver
[ADR-003](docs/adr/ADR-003-lectura-directa.md)).

Salió del monolito para que mobile-bff y notifications no tuvieran que pegarle a ledger-core en
cada pantalla de "mis cuentas". La extracción quedó a mitad de camino: en vez de pedirle los
datos a ledger-core por su API, accounts-api tiene su propio pool a Postgres `ledger` y hace
sus queries ahí, join incluido. No es ideal. Está en el ADR por qué, y qué hace falta para
sacarlo.

## Qué expone

Todo bajo `/v1`, JWT de Keycloak salvo donde se aclara:

- `GET /v1/accounts/{id}` — cuenta + customer (join).
- `GET /v1/customers/{id}/accounts` — cuentas de un customer.
- `GET /v1/accounts/{id}/statement?from=&to=` — movimientos (`ledger_entries`).
- `POST /v1/accounts/{id}/freeze` — requiere rol `service`. `UPDATE` directo sobre
  `accounts.status`.

`GET /health` y `GET /metrics` sin auth.

## Correr local

```
docker compose up
```

Levanta un Postgres vacío y accounts-api apuntando a él. Alcanza para ver `/health` en verde;
para datos reales necesitás correr las migraciones de ledger-core contra esa misma base primero
— accounts-api no trae las suyas, no es dueño del schema.

## Tests

```
go test ./...
```

Los handlers se testean contra un `store` fake (`internal/http/handlers_test.go`), sin Postgres
de por medio.

## Deploy

Helm, vía Actions (`deploy.yml`): push a `main` sube a staging con el `sha` corto de la imagen;
un tag `v*` sube lo mismo a prod. Chart en `deploy/chart`, valores por entorno en
`deploy/values-staging.yaml` y `deploy/values-prod.yaml`.

```
helm upgrade --install accounts-api ./deploy/chart -n staging -f deploy/values-staging.yaml
```

— Julián
