# ADR-0006: Persistir en Postgres con pgvector y migrar al arrancar

- **Estado:** aceptado
- **Fecha:** 2026-10-11
- **Issue relacionado:** #23

## Contexto

El sistema necesita guardar tres cosas: los documentos del corpus y el
estado de su ingesta, los fragmentos de cada documento con su ubicación, y
el vector de cada fragmento para la búsqueda semántica.

Las fuerzas relevantes:

- Una cita debe poder seguirse desde el vector recuperado hasta el
  documento y la sección de origen. Fragmento, vector y documento son un
  mismo dato visto desde tres lados.
- El presupuesto es inferior a un dólar mensual y el servicio escala a
  cero. La base de datos tiene que poder hacer lo mismo.
- El corpus es pequeño: una RCA con sus adendas e informes son miles de
  fragmentos, no millones.
- Un solo autor y un solo binario ([ADR-0005](0005-monolito-modular.md)).
  No hay un paso de despliegue con acceso a la base de datos, ni quien lo
  opere.
- La imagen de producción es distroless: no hay shell ni herramientas con
  las que ejecutar una migración a mano dentro del contenedor.

## Decisión

Usaremos Postgres con la extensión pgvector como único almacén, alojado en
Neon, y el binario aplicará las migraciones pendientes al arrancar.

**Un solo almacén.** Documentos, trabajos, fragmentos y vectores viven en la
misma base de datos. La relación entre un vector y su documento es una
clave foránea, y registrar un documento con su trabajo es una transacción.

**Neon en su plan gratuito.** Suspende el cómputo cuando no hay actividad,
igual que Cloud Run, y no exige medio de pago. La cadena de conexión vive en
Secret Manager y llega al proceso en `DATABASE_URL`; el workflow de
despliegue solo conoce el nombre del secreto.

**Conexión directa, no por el pooler de Neon.** El servicio tiene un tope de
tres instancias con cuatro conexiones cada una, muy por debajo del límite de
Neon. El pooler agregaría un modo de transacción con restricciones propias a
cambio de una capacidad que no se necesita.

**Migraciones SQL embebidas, aplicadas al arrancar.** Cada módulo es dueño
de su esquema y embebe sus migraciones; hoy solo `rag` tiene. Se aplican con
goose antes de aceptar tráfico, bajo un bloqueo de sesión de Postgres: si
dos instancias arrancan a la vez, una migra y la otra espera y encuentra el
esquema al día.

**Fallar al arrancar.** Con `DATABASE_URL` definida, no poder conectar o
migrar impide que el proceso arranque. Cloud Run no envía tráfico a una
revisión que no arranca, así que un despliegue con una migración rota deja
sirviendo a la revisión anterior. Sin `DATABASE_URL`, el proceso arranca sin
base de datos, lo que mantiene el desarrollo local sin dependencias.

**pgx como driver**, sin ORM ni generador de consultas. Las consultas son
SQL escrito a mano en el subpaquete de persistencia de cada módulo.

## Alternativas consideradas

### Una base de datos vectorial dedicada

Qdrant, Pinecone o similares, junto a un Postgres para los metadatos.

Ofrecen índices y filtros más sofisticados a gran escala. Se descartó
porque parte en dos sistemas un dato que debe ser consistente: borrar un
documento exigiría borrar en ambos sin transacción que los una, y una cita
podría apuntar a un fragmento que ya no existe. A la escala de este corpus,
pgvector resuelve la búsqueda sin esa complejidad.

### Cloud SQL

Postgres administrado en la misma nube que el servicio, con red privada.

Se descartó por costo: no escala a cero, y su instancia más pequeña supera
por sí sola varias veces el presupuesto mensual del proyecto.

### Migrar en el despliegue, no en el arranque

Un paso del workflow que aplica las migraciones antes de desplegar la
revisión nueva. Es lo habitual en equipos, porque separa "cambiar el
esquema" de "arrancar el servicio" y permite revisar la migración aparte.

Se descartó porque exige que GitHub Actions alcance la base de datos y
tenga su credencial, lo que amplía dónde vive el secreto, y porque agrega
un paso que alguien debe vigilar. Es la alternativa a reconsiderar primero.

### Un ejecutor de migraciones propio

Aplicar los archivos SQL con unas decenas de líneas, sin goose.

Evitaría una dependencia que pesa en la imagen. Se descartó porque el
bloqueo entre instancias y el registro de versiones son justamente la
parte que no conviene reescribir.

## Consecuencias

### Positivas

- Borrar un documento elimina sus trabajos, y más adelante sus fragmentos y
  vectores, en una sola operación consistente.
- La base de datos no tiene costo y no puede generarlo: no hay medio de
  pago registrado.
- El esquema desplegado siempre corresponde al código desplegado.
- Los tests de persistencia corren contra un Postgres real con pgvector, en
  local y en CI, cada paquete en un esquema propio.

### Negativas

- **Dos arranques en frío encadenados.** Si Cloud Run y Neon están ambos
  suspendidos, la primera petición espera a que despierten los dos. Neon
  tarda del orden de un segundo o más; no está medido todavía.
- **Una caída de Neon impide arrancar instancias nuevas**, aunque hoy
  ningún endpoint use la base de datos. La degradación controlada que el
  proyecto busca no existe para esta dependencia. Se revisa con el endpoint
  de readiness (#7).
- **Las migraciones no tienen vuelta atrás automática.** Los archivos
  declaran su `Down`, pero nada lo ejecuta: una migración destructiva
  aplicada en el arranque no se revierte desplegando la revisión anterior.
  Toda migración debe ser compatible con el código de la revisión previa.
- **El proceso de la aplicación puede alterar el esquema.** El mismo rol de
  base de datos que sirve las consultas crea y elimina tablas. Separar un
  rol de migración de uno de ejecución queda pendiente.
- **La imagen pasa de 8,4 a 13,5 MB** por pgx y goose.
- **goose 3.28 exige Go 1.26**, por encima del techo que fija
  [ADR-0002](0002-tool-versions.md). Se usa la 3.27.3, y al agregarla la
  directiva `go` del módulo subió de 1.25 a 1.25.7 por requisito
  transitivo. Al intentar la versión más nueva, Go descargó otro toolchain
  sin avisar; desde ahora `GOTOOLCHAIN=local` lo impide en el Makefile y en
  CI.
- **La versión de la imagen de Postgres está duplicada** en el Makefile y en
  el workflow de CI, la misma duplicación que ADR-0002 registra para las
  demás herramientas.
- **Neon no fija la versión menor de pgvector.** Los tests usan la 0.8.7;
  la que ofrezca Neon puede diferir.
- **`make test` ahora requiere Docker.** Sin él, los tests de persistencia
  no corren en local.

### Neutras

- La tabla de fragmentos y su columna de vectores no se crean todavía: la
  dimensión la fija el modelo de embeddings, que se elige en #32.
- Hay una sola tabla de versiones de goose. Si un segundo módulo llega a
  tener migraciones propias, habrá que darle la suya. *(Actualización,
  #66: `orchestrator` tiene la suya, `orchestrator_goose_version`.)*
- Los identificadores son UUID versión 7, generados por la aplicación:
  ordenables por fecha de creación, lo que favorece al índice y a la
  paginación por cursor.

## Revisión

Reconsiderar el almacén único si el corpus crece hasta que pgvector no
responda en el tiempo que la consulta tolera, o si se necesitan filtros que
no pueda expresar.

Reconsiderar la migración al arrancar si aparece una migración que no pueda
ser compatible con la revisión anterior, o si el tiempo de arranque se
vuelve relevante. La alternativa a evaluar es migrar como paso del
despliegue.

Reconsiderar Neon si su plan gratuito cambia de condiciones.
