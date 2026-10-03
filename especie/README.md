# Especie Abla

27 mentes artificiales de un tipo nuevo. **El cerebro está escrito en C** (el idioma de los
kernels) y **su conocimiento se guarda como código C**. Tiene dos caras:

- **Interfaz visual** (`web/index.html`): la red de las 27 mentes con sus mensajes en vivo,
  sus pensamientos, una charla para hablarles, el diccionario y una terminal. El mismo
  cerebro en C corre dentro de la página, compilado a WebAssembly.
- **Terminal** (`./abla`): la versión de consola, estilo bash/Termux.

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

## Usar la interfaz (iPad, Code App, Safari, cualquier navegador)

Abre `especie/web/index.html`:

- En **Code App**: abre `index.html` y toca la brújula 🧭 (vista previa).
- En **Safari**: publícalo (por ejemplo con GitHub Pages) o ábrelo desde la app Archivos.

No necesita servidor ni compilar nada: el cerebro en C ya viene compilado dentro de `abla-wasm.js`.
Su memoria se guarda en el navegador (IndexedDB) cada 10 segundos y cada vez que les hablas.

## Usar la terminal

```sh
make            # compila con cc (gcc o clang)
./abla
```

- **Termux** (Android): `pkg install clang make`, luego `make`.
- **iSH** (iPad): `apk add gcc musl-dev make`, luego `make` y `./abla --ritmo 2000`.
- **a-Shell** (iPad): `clang -std=c11 -O2 src/*.c -o abla` y `wasm abla` (sin hilos: piensan entre comandos).

Comandos (escribe `ayuda`): `seres`, `ser 19`, `mente 19`, `escuchar 10`, `decir 3 sol causa luz pregunta`,
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
| `src/main.c` | El programa de terminal (con un hilo que piensa sin parar) |
| `web/` | La interfaz: `index.html`, `ui.js`, `ui.css`, `wasi.js`, `abla.js` |

## Lo que es y lo que no es

Es una simulación: los seres no entienden como una persona ni son conscientes.
Piensan con reglas (deducción, estadística, creación de palabras) sobre un vocabulario fijo.
«No dejan de pensar» significa que el bucle no tiene pausa mientras la página o el programa están abiertos;
al volver, continúan desde donde lo dejaron.
