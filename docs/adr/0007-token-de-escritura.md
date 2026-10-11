# ADR-0007: Proteger las escrituras con un token único

- **Estado:** aceptado
- **Fecha:** 2026-10-11
- **Issue relacionado:** #36

## Contexto

Desde que los endpoints de documentos operan sobre la base de datos, la
demo pública acepta escrituras de cualquiera. El servicio corre en Cloud
Run con `--allow-unauthenticated`, y debe seguir así: el proyecto existe
para que alguien pueda abrirlo y probarlo sin pedir acceso.

El riesgo no es de costo. La base de datos está en un plan gratuito sin
medio de pago. Es de disponibilidad: ese plan tiene 1 GB, cada documento
admite 64 KiB de metadatos, y unas dieciséis mil peticiones lo llenan.
Además, cualquiera puede eliminar el corpus que la demo muestra.

El contrato declara desde el inicio un esquema `bearerAuth` con formato
JWT, aplicado a toda la API, que nunca se implementó. La autenticación
completa —identidad, emisión y expiración de tokens— está prevista para
más adelante y no hay todavía usuarios que distinguir.

## Decisión

Las operaciones que modifican el corpus exigirán un token único de
escritura; leer y consultar seguirá siendo público.

El token es una cadena opaca que el servicio recibe en `API_WRITE_TOKEN`,
desde Secret Manager, y que el cliente presenta en `Authorization: Bearer`.
No identifica a nadie: quien lo tiene puede escribir.

Un servicio sin token configurado rechaza toda escritura. La alternativa,
aceptarlas, convertiría un error de configuración en una demo abierta.

El contrato deja de declarar seguridad global y la declara por operación,
en `createDocument` y `deleteDocument`. El esquema deja de anunciarse como
JWT. La comparación del token se hace sobre hashes y en tiempo constante.

## Alternativas consideradas

### Implementar ya la autenticación con JWT

Es lo que el contrato anunciaba. Se descartó por ahora porque exige decidir
quién emite los tokens, cómo se rotan las claves y qué identidades existen,
para un sistema que tiene un solo operador. Sería construir un sistema de
identidad para proteger dos endpoints.

### Limitar la cantidad de peticiones por origen

Frena el abuso sin pedir credenciales. Se descartó como única defensa: con
instancias que escalan a cero y hasta tres en paralelo, un límite en
memoria se reinicia y se reparte; hacerlo bien requiere estado compartido.
Y no impide borrar: una sola petición elimina un documento.

Sigue siendo necesario para las consultas cuando empiecen a consumir
tokens de un proveedor, y se tratará entonces.

### Un tope de documentos

Impide llenar la base de datos, pero no vaciarla, y un tercero podría
agotar el tope y bloquear al operador.

### Quitar el acceso público a Cloud Run

Protege todo, incluida la lectura. Se descartó porque anula el propósito
de la demo.

## Consecuencias

### Positivas

- Un tercero no puede llenar ni vaciar la demo.
- La API sigue siendo explorable sin credenciales, y Swagger UI permite
  ingresar el token para probar las escrituras.
- El contrato vuelve a decir la verdad sobre qué exige autenticación.

### Negativas

- **Un solo token, compartido y sin expiración.** No hay forma de revocar
  el acceso de alguien sin cambiarlo para todos, ni de saber quién hizo
  qué. Si se filtra, hay que rotarlo a mano en Secret Manager y volver a
  desplegar.
- **La lista de operaciones protegidas está en dos lugares**: el contrato
  y `protectedOperations`. El código generado no expone los requisitos de
  seguridad. Un test compara ambas listas, pero lee el YAML como texto y
  depende de su formato.
- **Una operación nueva que escriba queda abierta por omisión** si nadie la
  declara protegida. El test no detecta eso: solo que ambos lados
  coincidan.
- **No hay límite de intentos.** El token tiene 256 bits de entropía, lo
  que hace inviable adivinarlo, pero nada registra ni frena los intentos.
- **Las consultas siguen abiertas.** Hoy responden datos de ejemplo sin
  costo; cuando llamen a un modelo, cada petición anónima consumirá saldo.

### Neutras

- El esquema sigue llamándose `bearerAuth` en el contrato, lo que deja el
  camino abierto para que más adelante transporte un JWT sin cambiar el
  nombre.

## Revisión

Reconsiderar cuando exista un segundo operador, cuando haga falta
distinguir quién escribió, o cuando las consultas tengan costo y deban
limitarse por cliente. En cualquiera de esos casos corresponde la
autenticación con identidad que este ADR posterga.
