# Nyx — el cerebro de las 18 IAs, en C

El cerebro resonante de las IAs de Nyxshell, llevado a C para correr en
**Code App** (iPad) o en cualquier terminal. La interfaz también es en C.

## Archivos

| archivo | qué es |
|---|---|
| `nyx.c` | la interfaz (el que ejecutas) |
| `cerebro.h` | el cerebro de las 18 mentes |
| `resh.h` | la lengua Resh: 3144 palabras + partículas |
| `libro.txt` | un librito para enseñarles (`lee libro.txt`) |

Los 4 tienen que estar en la **misma carpeta**.

## Cómo correrlo

- **Code App:** abre `nyx.c` y pulsa ▶.
- **Terminal:** `clang nyx.c -o nyx -lm && ./nyx`

La memoria se guarda en `nyx_memoria.txt` (al salir y cada 15 órdenes).

## Órdenes

```
qué es la luz            las 18 deliberan y responde la que más convence
@creativo el mar         hablas con una sola
bien · mal               tu opinión: refuerza o inhibe esa respuesta
vivo 30                  verlas vivir: imaginan, hablan, se enseñan Resh y aprenden
conversa narrativa empatia 8
enseña el gato duerme al sol
lee libro.txt
resh árbol               árbol → yuz, y cuántas lo saben decir
mentes                   las 18 de un vistazo
mente curiosidad         una por dentro: lo que aprendió a preferir, su foco, sus recuerdos
ayuda · salir
```

## Qué mejoró respecto al cerebro en Swift

- **Frases de verdad:** cada acople guarda también el orden de las palabras,
  así responden con frases en español aprendidas de lo que leen y oyen.
- **Responden a la pregunta:** manda lo que la pregunta evoca; las palabras
  vacías (el, la, de, qué…) ya no ganan los veredictos.
- **Resh repartido:** cada una sabe una parte, pregunta lo que no sabe
  ("ye mundo?") y otra le contesta ("mundo ka yen").
- **Imaginación con aprendizaje:** en cada paso una mente imagina qué quiere
  hacer (hablar, preguntar, imaginar, soñar, recordar, escuchar), lo hace,
  mide cómo le fue y ajusta cuánto le conviene. `mente <nombre>` lo muestra.
- **Tu opinión cuenta:** `mal` pone esa respuesta en oposición de fase y la
  próxima vez gana otra; `bien` refuerza la idea y la frase.
- **Fatiga y anti-eco:** lo que acaban de decir descansa, así las
  conversaciones avanzan en vez de repetirse.
- **Gramática sana:** el orden de las palabras se aprende sobre todo de ti y
  de los libros; de las otras mentes solo un poco (si no, copiarían sus frases
  a medio hacer).
