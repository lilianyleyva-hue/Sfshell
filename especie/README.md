# Especie Abla

27 mentes artificiales de un tipo nuevo. **El cerebro está escrito en C** (el idioma de los
kernels) y **su conocimiento se guarda como código C**. Tiene dos caras:

- **Interfaz de terminal en C** (`./abla`): pantalla completa con la red de las 27 mentes y sus
  mensajes en vivo, sus pensamientos, la ficha de cada ser con su conocimiento en C, y una línea
  de órdenes estilo bash/Termux. Todo en C, sin dependencias.
- **La misma interfaz de terminal dentro de una página** (`web/index.html`), para **Code App** y Safari:
  `tui.c` y todo el cerebro compilados a WebAssembly; la página solo muestra lo que dibuja el C y le
  pasa las teclas. Piensa rápido (un ciclo cada 0,3 s, o hasta 0,05 s con ⏩).
- Hay además una vista gráfica (`web/visual.html`) con la red dibujada.

No son modelos de lenguaje: son **agentes simbólicos**. Razonan con hechos y reglas, recuerdan,
se hablan entre ellos y **no dejan de pensar** mientras la página o el programa están abiertos.

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

## Usarla en Code App (iPad) o en cualquier navegador

1. En Code App, abre `especie/web/index.html`.
2. Toca la brújula 🧭 (vista previa).

Es la interfaz de terminal en C, igual que en iSH, pero rápida. Toca la pantalla para escribir.
La barra de abajo trae las teclas que el teclado del iPad no tiene (Tab, flechas, RePág, Esc),
y ⏪ ⏩ para cambiar la velocidad de pensamiento.

No necesita servidor ni compilar nada: el cerebro en C ya viene compilado dentro de `abla-wasm.js`.
Su memoria se guarda en el navegador (IndexedDB) cada 10 segundos y cada vez que les hablas.

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

Dónde funciona:

- **iSH** (iPad, gratis en la App Store): `apk add gcc musl-dev make git`, clona el repositorio, `cd Sfshell/especie`, `make` y `./abla`.
  iSH es lento: si va pesado, `./abla --ritmo 2000`. Pon el iPad en horizontal para ver la red (necesita 96 columnas o más).
- **Termux** (Android): `pkg install clang make`, luego `make` y `./abla`.
- **Linux y macOS**: `make` y `./abla`.
- **a-Shell** (iPad): no tiene terminal cruda, así que usa la consola de líneas: `clang -std=c11 -O2 src/*.c -o abla` y `wasm abla`.
- **Code App**: su terminal no muestra la salida de los programas; abre `web/index.html` con la 🧭 (es esta misma interfaz).

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
| `web/index.html`, `terminal.js` | La interfaz de terminal en una página (muestra lo que dibuja `tui.c`) |
| `web/visual.html`, `ui.js` | La vista gráfica |
| `web/wasi.js`, `abla.js` | Sistema de archivos para el cerebro en el navegador, y el puente con el C |

## Lo que es y lo que no es

Es una simulación: los seres no entienden como una persona ni son conscientes.
Piensan con reglas (deducción, estadística, creación de palabras) sobre un vocabulario fijo.
«No dejan de pensar» significa que el bucle no tiene pausa mientras la página o el programa están abiertos;
al volver, continúan desde donde lo dejaron.
