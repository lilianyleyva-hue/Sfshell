# Especie Abla

27 mentes artificiales de un tipo nuevo, en C++17 sin dependencias. No son
modelos de lenguaje: son **agentes simbólicos**. Razonan con hechos y reglas, recuerdan,
se hablan entre ellos y **no dejan de pensar** mientras el programa está abierto.

## Las 3 tribus (9 seres cada una)

| Tribu | Qué hace |
|---|---|
| **Lenguaje** | Traduce lo que sabe a frases de Abla, conversa y **crea palabras nuevas** cuando dos conceptos aparecen juntos a menudo (p. ej. «vitash» = luz + sombra), y se las enseña a toda la especie. |
| **Datos** | Mide y cuenta su conocimiento, encuentra el concepto más conectado y reparte **data C++ exacta** a los demás. |
| **Lógica** | Deduce hechos nuevos («voz es sonido» y «sonido es señal» ⇒ «voz es señal»), detecta contradicciones y pregunta cuando duda. |

Cada ser nace con una parte distinta del saber, así que para saberlo todo **tienen que hablar**.

## Abla, el idioma con el que nacen

- **4000 palabras innatas** = 200 raíces × 20 aspectos (muchos, negado, pregunta, pasado, futuro, quien, lugar…).
  Todos las conocen desde que despiertan.
- Una palabra es una raíz de 2 sílabas + un sufijo de aspecto: `tute` = palabra, `tutedi` = palabras, `tuteje` = quien hace palabras.
  Los nombres de los seres son palabras de Abla con el aspecto «quien» (Tuteje = «quien palabra»).
- Frase: `sujeto relación objeto [marca]`. Por ejemplo, `mapa pavu vimo` = «sol causa luz»; añadiendo `vari` (pregunta) se convierte en una pregunta.
- Diccionario completo: `mundo/abla/diccionario.txt`.

Se comunican de dos maneras: **en Abla** (aproximado, con duda o pregunta) o **en data C++**
(un `struct Hecho` exacto que no pasa por el idioma).

## Memoria y contexto

- **Contexto**: 1.000.000 de tokens por ser por defecto (`--contexto N`), en RAM.
- **Largo plazo**: todo lo que piensan se guarda en `mundo/memoria/<Nombre>.mem` y nunca se borra; `recordar` busca también ahí.
- **Conocimiento en C++**: `mundo/conocimiento/<Nombre>.cpp` es un arreglo `const Hecho Nombre[]`
  que compila de verdad, y al despertar cada ser vuelve a leer de ahí lo que sabe.

## Compilar y usar

```sh
make            # o: g++ -std=c++17 -O2 -pthread src/main.cpp -o abla
./abla
```

En **Termux** (Android): `pkg install clang make` y luego `make CXX=clang++`.

En **iPad (iPadOS 18)**: instala **iSH Shell** desde la App Store y ejecuta:

```sh
apk add g++ make
make
./abla
```

iSH emula x86, así que compila despacio y piensa más lento; si va pesado, usa `./abla --ritmo 2000`.

## La terminal

Al abrirla tienes un shell estilo bash dentro de `mundo/`: `ls`, `cd`, `cat`, `mkdir`, `echo > archivo`, `tree`, `grep`…
Cada ser tiene su propia terminal y su `$HOME` en `/seres/<Nombre>`, donde escribe sus archivos
(`teoremas.txt`, `datos.csv`, `palabras.abla`, `diario.abla`). Con `terminal <ser>` ves qué comandos ha ejecutado.

Comandos de la especie (escribe `ayuda`):

```
seres                         los 27 y qué piensan
ser 19                        memoria, contexto y saber de un ser
escuchar 10                   escucha a la especie pensar en vivo
decir 3 sol causa luz pregunta      háblale (en Abla o con raíces en español)
enseñar todos fuego causa caliente  mándales data C++ exacta
alimentar todos hechos.txt    un archivo de líneas «a es|parte|causa|igual|opuesto b»
dic palabra · traducir mapa pavu vimo · idioma · red · guardar
```

## Lo que es y lo que no es

Es una simulación: los seres no entienden como una persona ni son conscientes.
Piensan con reglas (deducción, estadística, creación de palabras) sobre un vocabulario fijo.
«No dejan de pensar» significa que el bucle no tiene pausa mientras el programa está abierto;
al cerrarlo se guarda todo, y al volver a abrirlo continúan desde donde lo dejaron.
