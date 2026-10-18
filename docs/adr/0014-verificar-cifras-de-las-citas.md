# ADR-0014: Verificar que las cifras de una respuesta estén en lo que cita, y avisar si no

- **Estado:** aceptado
- **Fecha:** 2026-10-18
- **Issue relacionado:** #74

## Contexto

Una respuesta se entrega solo si sus citas pasan una verificación de
forma: cada marca `[n]` apunta a un pasaje entregado, y hay al menos una
([ADR-0011](0011-recuperacion-y-modo-degradado.md),
[ADR-0012](0012-generacion-gemini-flash-lite.md)). Nada comprueba que el
pasaje diga lo que la oración afirma. La evaluación ya mostró respuestas
que citan pasajes reales pero tangenciales (#58) y enumeraciones que
agregan un elemento de otra lista (#61).

En un sector fiscalizado, el error más dañino es una cifra equivocada: un
plazo de notificación, un límite de emisión, una capacidad. Una cifra se
puede buscar en el texto citado sin otro modelo y sin gastar tokens.

Antes de decidir se midió el problema sobre las 116 respuestas distintas
que produjeron las corridas de evaluación guardadas desde #51:

- **238 cifras** aparecen en oraciones con cita.
- **Ninguna** es una cifra que el pasaje citado no contenga. Con
  temperatura 0 y este corpus, el modelo no inventó cifras.
- Una verificación ingenua, que compara el texto tal cual, marcó **dos
  falsos positivos**. En uno, la respuesta decía "1 de agosto de 2024" y
  el pasaje, "01 de agosto". En el otro, la abreviatura "D.S." partió una
  oración y una cifra de la pregunta, negada por el modelo, quedó del lado
  que tenía citas.
- Las únicas cifras sin respaldo están en frases que niegan una premisa de
  la pregunta ("no contienen información sobre una capacidad de 4 m³"),
  que no llevan marcas.

El riesgo, entonces, no se ha observado; la verificación sería un seguro.
Y el costo de un seguro mal calibrado está medido: con la comparación
ingenua, dos de 116 respuestas correctas se habrían tratado como
erróneas.

## Decisión

Verificaremos que cada cifra de una oración con cita aparezca en los
pasajes que esa oración cita; si alguna no aparece, la respuesta se
entrega igual, con un aviso explícito que nombra la cifra, y la traza lo
declara.

**Tres niveles de verificación.** El sistema distingue:

1. *Forma:* cada marca apunta a un pasaje entregado. Ya existe; si falla,
   la respuesta se descarta.
2. *Cifras:* cada cifra de una oración con marcas está en alguno de los
   pasajes que esa oración cita. Es lo que agrega este ADR.
3. *Texto:* el pasaje dice lo que la oración afirma. No se verifica.

**Qué es una cifra.** Una secuencia de dígitos, con separadores de miles o
decimales. Se comparan normalizadas: sin ceros a la izquierda, con el
punto de miles quitado y la coma decimal como punto. "4.000" y "4000" son
la misma cifra; "1" y "01" también. Las unidades no se comparan: "24
horas" y "24 h" coinciden porque coincide el 24. Comparar unidades
exigiría un catálogo, y lo que se quiere detectar es el número cambiado.

**Qué no se verifica.**

- Las oraciones sin marcas, que el sistema ya trata aparte: las frases de
  ausencia, que suelen repetir la cifra de la pregunta para negarla.
- Las marcas de cita, que se quitan antes de buscar cifras.

Los numerales de sección ("12.1") y los números de una norma ("D.S.
N° 90/2000", "artículo 138") sí se verifican como cualquier cifra: si la
oración los cita, el pasaje debería contenerlos.

**Avisar, no descartar.** Si una oración tiene una cifra que no está en lo
que cita:

- la respuesta se entrega, y al final del texto se agrega un aviso que
  nombra cada cifra sin respaldo e invita a verificarla en las citas;
- la traza la declara en una lista de cifras sin respaldo.

No se descarta la respuesta porque un falso positivo descartaría una
respuesta correcta, y la medición muestra que los falsos positivos son
más probables que los verdaderos. Tampoco se quita la oración, porque
quitarla en silencio cambia lo que la respuesta dice sin que quien
consulta lo sepa.

## Alternativas consideradas

### Un segundo modelo como juez

Pedirle a un modelo que lea cada oración y sus pasajes y diga si el pasaje
la sustenta. Es la única alternativa que verificaría el nivel de texto.
Se descartó por costo:

- Duplica la entrada de cada consulta: unos 3.600 tokens más, cerca de
  0,0011 USD a tarifa de lista ([ADR-0013](0013-costo-estimado-por-consulta.md)).
- Duplica las peticiones de generación, que en la capa gratuita tienen
  cuota diaria.
- Suma la latencia de otra generación: unos 600 ms medidos en producción.
- Un juez también se equivoca, y no hay cómo medir cuánto sin otro
  conjunto de evaluación.

Con cero cifras inventadas medidas, el gasto no se justifica hoy.

### Descartar la respuesta si una cifra no tiene respaldo

Es lo más seguro si la verificación fuera exacta. Se descartó porque no
lo es: con los dos falsos positivos medidos, dos respuestas correctas
habrían terminado en modo degradado. La normalización corrige esos dos
casos, pero aparecerán otros.

### Quitar la oración con la cifra sin respaldo

Entrega el resto de la respuesta. Se descartó porque cambia lo que la
respuesta dice sin avisar, y porque una oración quitada puede dejar a la
siguiente sin sentido.

### Comparar el embedding de cada oración con el de sus pasajes

Verificaría algo parecido al nivel de texto sin un modelo de generación.
Se descartó: gasta una petición de embeddings por oración, de la misma
cuota diaria que la búsqueda, y la similitud no distingue "24 horas" de
"48 horas", que es justamente el error que importa.

## Consecuencias

### Positivas

- Una cifra cambiada entre el pasaje y la respuesta queda a la vista de
  quien consulta, sin costo en tokens ni en latencia apreciable.
- La traza dice qué nivel de verificación pasó cada respuesta.

### Negativas

- **No detecta una cifra correcta en el lugar equivocado.** Si el pasaje
  dice "24 horas para la SMA y 48 horas para el informe" y la respuesta
  invierte los plazos, las dos cifras están en el pasaje y la
  verificación pasa.
- **No detecta errores sin cifras**, como el de las enumeraciones de #61.
- **Una respuesta con aviso se entrega igual.** Quien no lea el aviso
  puede tomar la cifra por verificada.
- **La normalización es una heurística.** Una fecha escrita como "1° de
  agosto", un número en palabras ("tres años") o un rango ("17-18") pueden
  producir falsos positivos o pasar sin verificar.
- **El beneficio no está medido en condiciones reales**, porque no hubo
  cifras inventadas que detectar. Se mide con preguntas que inducen a
  inventarlas (#76).

### Neutras

- El contrato agrega a la traza la lista de cifras sin respaldo; el
  texto de la respuesta lleva el aviso para quien no lee la traza.

## Revisión

Reconsiderar el juez si la evaluación muestra errores de texto que las
cifras no detectan y que importen más que el costo, o si el proyecto pasa
a la capa de pago y el costo deja de ser el límite. Reconsiderar
descartar en vez de avisar si los falsos positivos medidos llegan a cero
en una corrida amplia.
