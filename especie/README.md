# Especie Abla

27 mentes artificiales de un tipo nuevo. **El cerebro está escrito en C** (el idioma de los
kernels) y **su conocimiento se guarda como código C**. Resuelven **misiones de matemáticas** en
equipo para subir de nivel, y **miran fotos** en sus colores reales.

Tiene tres caras, y las tres usan el mismo cerebro en C:

- **La app** (`web/index.html`, para **Code App** en el iPad): toda la interfaz la **dibuja el C píxel
  a píxel** (`src/gui.c`, con su propia letra); la página solo copia esa imagen a la pantalla y le
  pasa los toques. Funciona sin internet: todo ocurre en el iPad.
- **La interfaz de terminal** (`./abla`, para iSH, Termux, Linux o macOS): pantalla completa con
  colores ANSI (`src/tui.c`). También en una página: `web/terminal.html`.
- **La consola** de líneas (`./abla --consola`).

No son modelos de lenguaje: son **agentes simbólicos**. Razonan con hechos y reglas, recuerdan,
se hablan entre ellos y **no dejan de pensar** mientras la app o el programa están abiertos.

## La app (iPad, en Code App)

1. En Code App, abre `especie/web/index.html`.
2. Toca la brújula 🧭.

Cinco pantallas, con la barra de pestañas abajo:

| Pantalla | Qué hay |
|---|---|
| **Especie** | La red de las 27 mentes con los mensajes viajando (● Abla, ■ data C) y lo que piensan. Toca una mente para ver su ficha, su mente y su conocimiento en C. |
| **Misiones** | El nivel, el problema que resuelven, quién lee, quién calcula y quién comprueba, lo que saben de cada operación, los últimos problemas y los logros. Puedes ponerles tú un problema. |
| **Visión** | La foto que están mirando en colores reales, lo que ven a su resolución, sus colores con el RGB real, brillo, contraste y bordes, y lo que les recuerda. **Añadir fotos** (puedes elegir cien de golpe) y **Mirar con la cámara**. |
| **Hablar** | Una charla con un ser o con toda la especie. |
| **Terminal** | Todos los comandos (`ayuda`). |

Arriba: ciclo, palabras, nivel, fotos en cola y la velocidad de pensamiento (− / +, desde 0,05 s por ciclo).

## Misiones de matemáticas

Cada problema lo resuelven entre todos, una fase por ciclo:

1. **Leer**: un ser del lenguaje lee el problema y lo dice en Abla.
2. **Calcular**: tres seres de datos lo calculan cada uno por su cuenta.
3. **Verificar**: dos seres de lógica comprueban cada resultado con la operación inversa.
4. **Decidir**: votan; si ninguna respuesta convence a la lógica, vuelven a calcular.
5. **Resultado**: si está bien, cuenta para subir de nivel. Acierten o fallen, todos los que participaron **aprenden**.

Cada ser tiene una habilidad para cada operación que sube con la práctica; al principio se equivocan
(sobre todo al multiplicar y en adelante) y van mejorando. Niveles: sumar, sumas grandes, restar,
multiplicar, dividir, potencias, ecuaciones, números primos, sucesiones, mezcla de todo y maestría.

**Premio de cada nivel: ven con más píxeles**: 64×48 → 96×72 → 128×96 → 160×120 → 192×144 → 256×192 → 320×240.
En el nivel 4 aprenden además palabras nuevas de matemáticas.

Comandos: `mision` (estado) y `mision 37*12` (ponerles un problema; `+ - * x / ^`).
Las cuentas "verdaderas" las hace el programa; los errores y el aprendizaje de los seres son simulados.

## Visión: mandarles fotos

Las fotos entran en `mundo/fotos/entrada/` como imágenes **PPM** (el formato más simple: los píxeles RGB
tal cual). La app convierte las que elijas (hasta 320×240) y las deja ahí; en la terminal puedes copiar
tus propios `.ppm`. La especie las mira **de una en una, un paso por ciclo**:

abrir → mirar (a su resolución) → colores (con su RGB real) → bordes (filtro de Sobel) → regiones (qué domina arriba y abajo) → describir (en español y en Abla) → compartir

Con 0,3 s por ciclo tardan unos 2 segundos por foto: **100 fotos, unos 4 minutos**. La app muestra cuántas
quedan y cuánto falta. Lo que aparece junto en las fotos lo asocia la tribu del lenguaje (y acaba creando
palabras), y si arriba y abajo tienen el mismo color, la lógica deduce que esas cosas se parecen.
Cada foto vista se guarda en pequeño en `mundo/fotos/vistas/` con su descripción. Comando: `fotos`.

## Las 3 tribus (9 seres cada una)

| Tribu | Qué hace |
|---|---|
| **Lenguaje** | Traduce lo que sabe a frases de Abla, conversa y **crea palabras nuevas** cuando dos conceptos aparecen juntos a menudo, y se las enseña a toda la especie. |
| **Datos** | Mide y cuenta su conocimiento, encuentra el concepto más conectado y reparte **data C exacta** (un `struct hecho`) a los demás. |
| **Lógica** | Deduce hechos nuevos («voz es sonido» y «sonido es señal» ⇒ «voz es señal»), detecta contradicciones y pregunta cuando duda. |

Cada ser nace con una parte distinta del saber, así que para saberlo todo **tienen que hablar**.

## Abla, el idioma con el que nacen

- **4000 palabras innatas** = 200 raíces × 20 aspectos (muchos, negado, pregunta, pasado, quien, lugar…).
  Todos las conocen desde que despiertan, y el idioma es idéntico en cualquier máquina.
- Una palabra es una raíz de 2 sílabas + un sufijo de aspecto: `sipe` = palabra, `sipegi` = palabras, `sipeje` = quien hace palabras.
  Los nombres de los seres son palabras de Abla con el aspecto «quien» (Sipeje = «quien palabra»).
- Frase: `sujeto relación objeto [marca]`. Por ejemplo, `nosa vure mika` = «sol causa luz»; con `piti` (pregunta) al final, es una pregunta.
- Diccionario completo: `mundo/abla/diccionario.txt` (o la pestaña Diccionario).

Se comunican de dos maneras: **en Abla** (aproximado, con duda o pregunta) o **en data C**
(un `struct hecho` binario exacto que no pasa por el idioma).

## Memoria y conocimiento

- **Contexto**: 1.000.000 de tokens por ser.
- **Largo plazo**: todo lo que piensan se guarda en `mundo/memoria/<Nombre>.mem`; `recordar` busca también ahí.
  En la terminal nunca se borra. En el navegador, donde el espacio es limitado, se guardan los últimos 256 KB por ser.
- **Conocimiento en C**: `mundo/conocimiento/<Nombre>.c` es un arreglo `const struct hecho Nombre[]`
  que compila de verdad con cualquier compilador de C, y al despertar cada ser vuelve a leer de ahí lo que sabe.
  En la interfaz, en la pestaña Ser, puedes verlo y descargarlo.

## Usar la interfaz de terminal

```sh
make            # compila con cc (gcc o clang)
./abla
```

```
┌ Red de la especie ──────────┐┌ [Pensamientos] Ser  Salida ─────── Tab: panel ┐
│   Rureje •    • Ronuje      ││ 12 Vikije: Deduzco que dato es parte de tabla… │
│ Sipeje •   ·  ▪   • Visije  ││ 13 Ronuje: Creo una palabra: «nisish» = raíz + │
│   …las 27 y sus mensajes    ││ palabra. Se la enseño a toda la especie.       │
└─────────────────────────────┘└────────────────────────────────────────────────┘
┌ Los 27 ───────────────────────────────────────────────────────────────────────┐
humano@abla:/$ decir 19 sol causa luz pregunta
```

- **Tab** cambia el panel de la derecha: Pensamientos (en vivo) · Ser (memoria, y su conocimiento en C) · Salida (lo que devuelven tus comandos).
- **← →** eligen un ser · **↑ ↓** historial · **RePág / AvPág** desplazan la salida · **Esc** borra la línea · `salir` o **Ctrl+C** para salir.
- Abajo escribes cualquier comando (`decir`, `enseñar`, `ls`, `cat`…). Mientras escribes, siguen pensando.
- `./abla --consola` usa la consola de líneas de siempre (también se usa sola si la entrada no es una terminal).
- `./abla --puente` es para que otro programa hable con la especie: cada línea que entra devuelve una línea JSON. Lo usa el [taller de Nyx y Abla](../nyx-abla/LEEME.md). Lo que ven en cada foto queda además en `fotos/vistas.jsonl`.

Dónde funciona:

- **iSH** (iPad, gratis en la App Store): `apk add gcc musl-dev make git`, clona el repositorio, `cd Sfshell/especie`, `make` y `./abla`.
  iSH es lento: si va pesado, `./abla --ritmo 2000`. Pon el iPad en horizontal para ver la red (necesita 96 columnas o más).
- **Termux** (Android): `pkg install clang make`, luego `make` y `./abla`.
- **Linux y macOS**: `make` y `./abla`.
- **a-Shell** (iPad): no tiene terminal cruda, así que usa la consola de líneas: `clang -std=c11 -O2 src/*.c -o abla` y `wasm abla`.
- **Code App**: su terminal no muestra la salida de los programas; abre `web/index.html` (la app) o `web/terminal.html` con la 🧭.

Comandos (escribe `ayuda`; también `ritmo 300` para que piensen más rápido): `seres`, `ser 19`, `mente 19`, `escuchar 10`, `decir 3 sol causa luz pregunta`,
`enseñar todos fuego causa caliente`, `alimentar todos hechos.txt`, `dic palabra`, `traducir nosa vure mika`,
`idioma`, `red`, `guardar`, y una terminal estilo bash dentro de `mundo/`: `ls`, `cd`, `cat`, `mkdir`, `echo > archivo`, `tree`, `grep`…
Cada ser tiene su propio `$HOME` en `/seres/<Nombre>`, donde escribe sus archivos (`teoremas.txt`, `datos.csv`, `palabras.abla`, `diario.abla`).

## Recompilar el cerebro para la web

```sh
make web        # clang con wasm32-wasi; en Ubuntu: apt install wasi-libc
```

Genera `web/abla.wasm` y lo empaqueta en `web/abla-wasm.js`. En el navegador, `web/wasi.js` le da al
cerebro un sistema de archivos en memoria para que sus `fopen`, `mkdir` y `opendir` funcionen igual que en disco.

## Código

| Archivo | Qué es |
|---|---|
| `src/idioma.c` | Abla: las 4000 palabras y las compuestas |
| `src/especie.c` | Los 27 seres, cómo piensa cada tribu y cómo se mandan mensajes |
| `src/saber.c` | Hechos con índice; exporta e importa el conocimiento como código C |
| `src/memoria.c` | Contexto en RAM y memoria de largo plazo en disco |
| `src/shell.c` | La terminal estilo bash (rutas encerradas en `mundo/`) |
| `src/comandos.c` | Los comandos y la API en JSON que usa la interfaz |
| `src/tui.c` | La interfaz de terminal a pantalla completa (ANSI + termios) |
| `src/main.c` | El programa de terminal: abre la interfaz, o la consola de líneas con un hilo que piensa sin parar |
| `src/misiones.c` | Las misiones de matemáticas, los niveles y el aprendizaje |
| `src/vision.c` | La visión: PPM, colores reales, bordes, regiones y la cola de fotos |
| `src/gui.c`, `src/gui_dibujo.c` | La app dibujada en C: pantallas, botones, círculos y texto con bordes suaves |
| `src/fuente.c` | La letra (DejaVu Sans) convertida a tablas de C por `herramientas/fuente.py` |
| `web/index.html`, `app.js` | La página de la app: copia la imagen que dibuja el C y le pasa toques, teclas, fotos y cámara |
| `web/terminal.html`, `terminal.js` | La interfaz de terminal en una página |
| `web/wasi.js`, `abla.js` | Sistema de archivos para el cerebro en el navegador, y el puente con el C |

## Lo que es y lo que no es

Es una simulación: los seres no entienden como una persona ni son conscientes.
Piensan con reglas (deducción, estadística, creación de palabras) sobre un vocabulario fijo.
«No dejan de pensar» significa que el bucle no tiene pausa mientras la página o el programa están abiertos;
al volver, continúan desde donde lo dejaron.
