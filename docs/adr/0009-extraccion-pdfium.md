# ADR-0009: Extraer el texto de los PDF con PDFium en WebAssembly

- **Estado:** aceptado
- **Fecha:** 2026-10-12
- **Issue relacionado:** #33

## Contexto

La ingesta empieza por obtener el texto de un PDF, página por página y en
orden de lectura. Todo lo que sigue —la fragmentación, los embeddings, las
citas— hereda la calidad de ese texto: un párrafo en desorden o una tabla
desarmada se convierten en respuestas que citan mal.

Las fuerzas relevantes:

- Los documentos del dominio son resoluciones firmadas del Servicio de
  Evaluación Ambiental. Traen un sello de firma incrustado en cada página,
  tablas extensas de dos columnas y, en su mayoría, capa de texto.
- La imagen de producción es distroless: contiene el binario y nada más.
  No hay intérprete de comandos ni programas auxiliares.
- El binario se compila sin CGO, lo que permite esa imagen y una
  compilación reproducible.
- El servicio procesa archivos que vienen de fuera. Un analizador de PDF es
  una superficie de ataque conocida.
- El servicio corre con 256 MiB de memoria y escala a cero.
- El proyecto está fijado en Go 1.25 ([ADR-0002](0002-tool-versions.md)).

Las alternativas se probaron contra un documento real: la RCA del proyecto
"Continuidad Operacional Minera Tres Valles", de 108 páginas.

## Decisión

Usaremos PDFium, el motor de PDF de Chromium, compilado a WebAssembly y
ejecutado dentro del proceso con wazero, a través de la librería go-pdfium.

El motor se inicia en la primera extracción y no al arrancar el proceso:
solo las instancias que ingieren un documento pagan su costo. Procesa un
documento a la vez.

Un PDF que no puede abrirse se rechaza como entrada inválida. Uno que abre
pero no entrega texto —un escaneo— se rechaza con un error propio, en lugar
de producir un documento ingerido sobre el que ninguna consulta encontraría
nada. El reconocimiento óptico de caracteres queda fuera de alcance.

### Mediciones

Sobre el documento de prueba, en la máquina de desarrollo:

| | Resultado | Memoria máxima | Tiempo |
|---|---|---|---|
| `ledongthuc/pdf` (Go puro) | Falla en las 108 páginas | — | — |
| PDFium en WebAssembly | 108 páginas, 58.003 palabras | 310 MiB | 4 s de inicio + 5 s |
| `pdftotext` (poppler) | 108 páginas, 57.477 palabras | 19 MiB | 0,5 s |

La memoria de PDFium es casi fija: 306 MiB para una página, 337 MiB para un
documento de 468 páginas. Ejecutar el WebAssembly con intérprete en lugar
de compilarlo no la reduce y multiplica el tiempo por diecisiete.

## Alternativas consideradas

### Una librería en Go puro

Es la opción sin costo: ni memoria, ni dependencias pesadas. Se probó
`ledongthuc/pdf`, la más usada.

Se descartó porque no puede leer estos documentos. Falla en todas las
páginas al encontrar la imagen incrustada del sello de firma. No es un caso
raro que pueda esquivarse: es el formato de toda resolución firmada del SEA.

### `pdftotext` como programa externo

Es la herramienta de referencia: rápida, liviana y con buen resultado.

Se descartó porque exige abandonar la imagen distroless por una con sistema
operativo y poppler, del orden de diez veces más grande, y ejecutar un
programa externo sobre archivos que vienen de fuera. Un fallo de memoria en
el analizador correría con los permisos del contenedor.

Es la alternativa a reconsiderar primero si la memoria de PDFium se vuelve
un problema.

### PDFium o MuPDF mediante CGO

Darían el mismo resultado con menos memoria. Se descartaron porque
reintroducen CGO y bibliotecas nativas en la imagen, que es lo que la
compilación estática evita, y porque MuPDF se distribuye bajo AGPL.

### Enviar el PDF a un modelo multimodal

El proveedor de embeddings puede leer PDF directamente. Se descartó porque
consume la cuota gratuita de la que depende la demo, no es determinista, y
no entrega la ubicación exacta de cada pasaje, que es lo que una cita
necesita.

## Consecuencias

### Positivas

- Lee los documentos reales del dominio, incluido su sello de firma, que
  deja fuera del texto.
- Entrega las tablas en orden de lectura: la etiqueta de cada fila seguida
  de su valor.
- El binario sigue siendo estático, sin CGO, en la misma imagen mínima.
- El analizador corre aislado: un fallo de memoria dentro del WebAssembly
  no alcanza la memoria del proceso.

### Negativas

- **Necesita unos 310 MiB, más que los 256 MiB del servicio.** Antes de
  conectar la ingesta hay que subir la memoria a 512 MiB (#34). El costo es
  de 0,2 centavos de dólar por hora de actividad, y solo una vez agotada la
  capa gratuita de Cloud Run.
- **Con esa memoria ocupada, una subida de 20 MiB y la extracción compiten
  en la misma instancia.** El margen debe medirse en #34.
- **Cuatro segundos de inicio en la primera extracción de cada instancia.**
  Como el servicio escala a cero, es probable que cada ingesta lo pague.
- **El binario crecerá cuando la extracción se conecte al servicio**: el
  motor compilado pesa 5 MB, más el entorno que lo ejecuta. El total no
  está medido. Hoy el binario no cambia: nada en él usa todavía la
  extracción.
- **La librería queda fijada en una versión antigua.** Desde la 1.20 exige
  Go 1.26; se usa la 1.19.8. Las correcciones de seguridad de PDFium
  posteriores no llegan mientras el techo de Go no suba. El aislamiento del
  WebAssembly lo atenúa, no lo elimina.
- **Los tests que usan el motor tardan cerca de un minuto cada paquete con
  el detector de carreras**, contra cinco segundos sin él. La verificación
  en CI pasa de uno a dos minutos.
- **Un documento escaneado se rechaza.** Una RCA antigua, anterior a la
  firma electrónica, no puede ingerirse.
- **El rechazo de escaneos está probado con un PDF sintético sin texto**,
  no con un escaneo real.
- **Se versiona un PDF de 1,4 MB** como documento de prueba.

### Neutras

- El umbral que distingue un escaneo —menos de cincuenta caracteres por
  página en promedio— es una estimación. Un documento mixto, con algunas
  páginas escaneadas, se acepta y esas páginas quedan sin texto.
- Las extracciones se serializan. Con una sola instancia y un solo
  operador que sube documentos, no hay espera real.

## Revisión

Reconsiderar si la memoria necesaria impide mantener el servicio dentro de
la capa gratuita, o si aparecen documentos que PDFium no lea bien. La
alternativa a evaluar es `pdftotext` en una imagen con sistema operativo,
o extraer la ingesta a un servicio propio, que es el candidato que
[ADR-0005](0005-monolito-modular.md) ya señalaba.

Actualizar la librería en cuanto el techo de Go lo permita.
