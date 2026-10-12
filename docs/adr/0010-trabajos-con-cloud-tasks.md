# ADR-0010: Ejecutar los trabajos en pasadas despachadas por Cloud Tasks

- **Estado:** aceptado
- **Fecha:** 2026-10-12
- **Issue relacionado:** #24

## Contexto

Generar los embeddings de un documento toma minutos: la RCA de prueba son
unos 112.000 tokens y la capa gratuita del proveedor admite 30.000 por
minuto ([ADR-0008](0008-embeddings-gemini.md)). El contrato de la API
responde `202 Accepted` al subir un documento y deja el resto a un trabajo
asíncrono.

El problema es dónde corre ese trabajo. Cloud Run, con la facturación por
petición que mantiene el servicio dentro de su capa gratuita, solo asigna
CPU mientras atiende una petición. Una goroutine que sigue trabajando
después de responder queda congelada hasta que llegue otra petición, y
termina cuando la instancia se apaga. [ADR-0005](0005-monolito-modular.md)
ya lo había anotado como consecuencia de mantener un solo binario.

Las fuerzas relevantes:

- El demo no debe generar un cobro, y el servicio debe seguir escalando a
  cero.
- Una petición tiene plazo: 120 segundos en este servicio.
- El proveedor de embeddings falla de forma transitoria y regula el ritmo.
  Un trabajo debe poder interrumpirse y continuar.
- Un solo operador y un solo binario: no hay un proceso aparte que vigile
  una cola.

## Decisión

El trabajo se ejecuta en pasadas, cada una dentro de una petición HTTP que
Cloud Tasks hace al propio servicio.

**Una operación ejecuta una pasada.** `POST /v1/jobs/{id}/run` genera los
embeddings de los fragmentos pendientes durante un minuto como máximo y
devuelve el estado del trabajo. Si quedan fragmentos, encola la pasada
siguiente antes de responder. Como el trabajo ocurre dentro de una
petición, tiene CPU.

**Cloud Tasks hace las peticiones.** Al registrar un documento se encola
una tarea que invoca esa operación. La cola despacha una tarea a la vez y
reintenta, con esperas de 30 segundos a 10 minutos, las que responden con
error.

**El avance vive en la base de datos, no en la tarea.** Cada grupo de
fragmentos se guarda apenas se obtiene, junto con el porcentaje de avance.
Una pasada no sabe qué hizo la anterior: pregunta qué fragmentos no tienen
vector y sigue desde ahí. Por eso una pasada interrumpida, repetida o
reintentada no pierde ni duplica trabajo.

**Una reserva con vencimiento impide dos pasadas simultáneas.** La pasada
marca el trabajo como reservado hasta una hora determinada. Otra que llegue
mientras tanto no hace nada. Si la pasada muere sin liberar, la reserva
vence sola.

**Los errores se dividen en dos.** Si el proveedor no responde, la pasada
devuelve 503 y la cola la reintenta más tarde; el trabajo sigue en curso.
Si el error no se arregla reintentando —el contenido fue rechazado, la
clave es inválida—, el trabajo se cierra como fallido con un mensaje para
quien lo consulta.

**La operación se protege con el token de escritura**
([ADR-0007](0007-token-de-escritura.md)), que la tarea lleva en un
encabezado. Queda en el contrato público: el operador puede usarla para
relanzar un trabajo que se detuvo.

**Fuera de Cloud Run, los trabajos corren dentro del proceso.** Sin cola
configurada, una goroutine ejecuta las pasadas. En local el proceso tiene
CPU todo el tiempo, y así el sistema completo funciona sin servicios de
Google.

## Alternativas consideradas

### Que la subida espere hasta terminar

La petición de subida haría todo y respondería al final. Es lo más simple:
no hay cola ni estado intermedio.

Se descartó porque la petición duraría cuatro minutos con el documento de
prueba y veinte con uno de 468 páginas, por encima del plazo de cualquier
cliente razonable, y porque convierte en síncrona una operación que el
contrato declara asíncrona.

### CPU siempre asignada

Cloud Run puede mantener la CPU fuera de las peticiones, con facturación
por instancia. Una goroutine bastaría.

Se descartó porque cambia el modelo de costo: la instancia se cobra
mientras exista, atienda o no peticiones, incluidos los minutos que tarda
en apagarse después de cada una. Es justamente lo que el presupuesto no
admite. Tampoco resuelve los reintentos ni la interrupción.

### Un servicio aparte para la ingesta

Un Cloud Run Job, o un segundo servicio, que procese los documentos. Es la
extracción que ADR-0005 prevé cuando un módulo necesita recursos distintos.

Se descartó por ahora porque agrega un segundo despliegue para resolver un
problema que una cola resuelve dentro del mismo binario. Sigue siendo el
camino si la ingesta crece.

### Avanzar el trabajo cuando el cliente consulta su estado

Cada `GET` del trabajo procesaría un grupo de fragmentos. No requiere
ningún servicio nuevo.

Se descartó porque el trabajo solo avanzaría mientras alguien lo mire, y
porque una lectura con efectos rompe lo que un cliente espera de un `GET`.

### Un programador que despierte al servicio cada minuto

Cloud Scheduler invocando una operación que revise si hay trabajo.

Se descartó porque despertaría a la base de datos cada minuto, haya o no
trabajo, y consumiría las horas de cómputo mensuales de su plan gratuito.

## Consecuencias

### Positivas

- El trabajo tiene CPU sin cambiar el modelo de costo: el servicio sigue
  escalando a cero y cobrando solo por petición.
- Es reanudable por construcción: interrumpir una pasada no pierde nada.
- Los reintentos ante un proveedor caído los hace la cola, con esperas
  crecientes, sin código propio.
- El operador puede relanzar un trabajo detenido con una petición.
- El sistema completo corre en local sin Google.

### Negativas

- **Un servicio más de Google del que depende la ingesta**, con su cola,
  su permiso y dos variables de entorno. Si Cloud Tasks no acepta la tarea,
  la subida se deshace y responde 503.
- **La cola se configuró a mano y no está en el repositorio.** Su ritmo,
  su concurrencia y sus reintentos no quedan versionados; quien replique el
  despliegue debe crearla con los mismos valores.
- **El token de escritura viaja dentro de cada tarea.** Quien pueda leer
  las tareas de la cola en el proyecto de Google puede leerlo. Rotar el
  token deja inservibles las tareas ya encoladas.
- **Si la cola agota sus diez reintentos, la tarea se descarta y el trabajo
  queda en curso para siempre**, sin que nada lo cierre ni avise. Se
  resuelve relanzándolo a mano.
- **El minuto de cada pasada es un límite blando.** Se revisa entre grupos
  de fragmentos; el grupo en curso termina aunque el plazo se cumpla, para
  no gastar cuota en una petición cancelada. Una pasada puede durar hasta
  un minuto y medio, bajo el plazo de 120 segundos.
- **Cada pasada mantiene activa la instancia y se factura como cualquier
  petición.** Ingerir la RCA de prueba son unos cuatro minutos de actividad,
  de las cincuenta horas mensuales gratuitas.
- **La dirección pública del servicio está escrita en el workflow de
  despliegue.** Si cambia, las tareas apuntarán a un servicio que no
  existe.
- **El despachador local y el de producción son código distinto.** Lo que
  se prueba en local no ejercita Cloud Tasks; esa parte se verifica solo en
  el servicio desplegado.
- **Un trabajo no puede cancelarse.** Eliminar el documento lo detiene de
  hecho: la pasada siguiente no encuentra el trabajo.

### Neutras

- La operación que ejecuta una pasada es idempotente: invocarla sobre un
  trabajo terminado o reservado devuelve su estado sin efecto.
- El despachador de Cloud Tasks vive en el binario y no en un módulo: es
  el único lugar que sabe a la vez qué ruta del gateway ejecuta un trabajo
  de `rag`.

## Revisión

Reconsiderar si la ingesta necesita más de lo que cabe en pasadas de un
minuto, si aparecen varios operadores subiendo documentos a la vez, o si el
costo de mantener activa la instancia durante las pasadas deja de caber en
la capa gratuita. La alternativa a evaluar es extraer la ingesta a un
servicio propio.
