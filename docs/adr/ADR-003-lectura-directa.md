# ADR-003: accounts-api lee la base `ledger` directamente

- **Estado**: aceptado
- **Fecha**: 2024-05
- **Autor**: Julián Ferreyra (core-banking)

## Contexto

accounts-api salió del monolito para que mobile-bff y notifications dejaran de pegarle a
ledger-core en cada pantalla de "mis cuentas". La idea original era que accounts-api le pidiera
los datos a ledger-core por su API igual que cualquier otro consumidor.

En la extracción, medimos la latencia de armar la pantalla de home vía `GET
/v1/accounts/{id}` + `GET /v1/accounts/{id}/statement` contra ledger-core y nos dio por encima
del budget que tenía mobile-bff para esa pantalla. ledger-core no tenía (todavía) un endpoint
de statement paginado ni un join `accounts`+`customers` en una sola llamada.

## Decisión

accounts-api se conecta directo a Postgres `ledger` (la base de ledger-core) con su propio
pool y hace `SELECT` sobre `accounts`, `customers` y `ledger_entries`, incluido el join para
`GET /v1/accounts/{id}`. `POST /v1/accounts/{id}/freeze` hace un `UPDATE accounts SET
status='frozen'` directo, en vez de pedirle a ledger-core que lo haga.

No es lo que queríamos: es un acoplamiento fuerte a un schema que no es nuestro y que
ledger-core puede cambiar sin avisarnos. Lo aceptamos por la fecha de entrega del H1 2024.

## Consecuencias

- accounts-api no tiene migraciones propias — no es dueño de ninguna tabla que toca.
- Cualquier cambio de schema en `ledger` (una columna, un rename) rompe accounts-api sin que
  ledger-core tenga forma de saberlo por un contrato explícito.
- El `UPDATE` de freeze bypassea cualquier regla de negocio que ledger-core le quiera poner al
  freezing de una cuenta a futuro (por ejemplo, no permitir congelar una cuenta con una
  transferencia in-flight).

## Plan

Esto es **temporal hasta que ledger-core exponga un endpoint de statement** (paginado, con
filtro por fecha) y un `GET /v1/accounts/{id}` que ya traiga el customer embebido. Cuando estén,
accounts-api pasa a consumir la API de ledger-core como cualquier otro servicio y este ADR se
marca superado. Trackeado informalmente, sin ticket — quedó pendiente de abrir uno.
