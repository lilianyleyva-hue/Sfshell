# Nyx Código

Nyx Código es un programa que se usa desde el navegador, en tu propio ordenador. Le hablas en español y:

- **escribe** funciones y programas en Go,
- **arregla** código que no compila o que da resultados malos,
- **explica** qué hace un código, línea a línea,
- **prueba** funciones y te da un archivo `_test.go`,
- **resuelve** problemas de matemáticas, de lógica y puzles.

No usa ninguna inteligencia artificial de lenguaje. Usa sus propios algoritmos y **comprueba cada
resultado antes de enseñártelo**. Junto a cada respuesta dice qué comprobó: por ejemplo «Cumple tus 3
ejemplos», «Comprobado sustituyendo» o «Sin comprobar». Si no ha comprobado algo, lo dice. Si una frase
sobre tu código no está comprobada, empieza por «Creo que…».

Mientras trabaja te enseña cómo piensa: qué entendió, qué probó y qué salió bien o mal.

## Qué necesitas

- **Linux**. Está pensado para Debian 13, pero sirve cualquier Linux.
- **Go 1.22 o más nuevo** (`go version` te dice cuál tienes). En Debian: `sudo apt install golang`.
  Go hace falta para compilar Nyx Código y también para que pueda compilar y probar tu código.
  Sin Go, la parte de matemáticas, lógica y puzles sigue funcionando.
- Un navegador (Firefox, Chromium…).

No necesita nada más: ni bases de datos, ni librerías de fuera, ni cuenta en ningún sitio.

## Instalar y abrir

```sh
cd ~/Sfshell/nyx-codigo && go build && ./nyx-codigo
```

Se abre una pestaña en tu navegador. En la terminal verás algo así:

```
Nyx Código está en http://127.0.0.1:8765/#t=… — si cierras la pestaña, vuelve a ejecutar nyx-codigo
```

Si cierras la pestaña, vuelve a ejecutar `./nyx-codigo`: abre otra vez la misma ventana (no arranca un
segundo programa). Para cerrarlo del todo, pulsa **Ctrl+C** en la terminal. Al cerrarse guarda la memoria.

La primera vez que compila código puede tardar entre 5 y 30 segundos, porque Go prepara su caché.
Después va mucho más rápido.

Si quieres tenerlo en el menú de aplicaciones:

```sh
./nyx-codigo -instalar
```

## Opciones

| Opción | Qué hace |
|---|---|
| `-puerto 8765` | El puerto local. Si está ocupado, usa otro libre. Con `0` elige uno cualquiera. |
| `-datos carpeta` | Dónde guarda su memoria (mira «Dónde guarda sus datos»). |
| `-sin-internet` | No usa internet para nada. |
| `-sin-navegador` | No abre el navegador; solo escribe la dirección en la terminal. |
| `-preguntar "…"` | Modo terminal: responde una sola pregunta, enseña cómo pensó y termina. |
| `-codigo archivo.go` | Junto con `-preguntar`: el código sobre el que preguntas. |
| `-instalar` | Crea el acceso directo `~/.local/share/applications/nyx-codigo.desktop`. |
| `-version` | Dice la versión. |

Ejemplos del modo terminal:

```sh
./nyx-codigo -preguntar "resuelve 2x+3=11"
./nyx-codigo -preguntar "haz una función que sume los números pares de una lista"
./nyx-codigo -preguntar "¿por qué falla?" -codigo main.go
```

Si en el modo terminal te hace una pregunta, contesta escribiendo el número de una opción (o tu
respuesta) y pulsa Intro. El programa termina con código 0 si lo consiguió y con 1 si no.

## Qué sabe hacer

- **Crear funciones** a partir de una frase: «haz una función que diga si una palabra es palíndromo».
- **Crear funciones a partir de ejemplos**: «[1,2,3,4] -> 6; [5,7] -> 0; [] -> 0». Si dos soluciones
  cumplen tus ejemplos pero no hacen lo mismo, te pregunta cuál quieres.
- **Crear programas completos**: «haz un programa que pida dos números y los sume», «una calculadora»,
  «adivina el número», «tabla de multiplicar del 7».
- **Arreglar código**: «no compila», «debería dar el máximo pero con [3,1,2] da 1», «me sale index out
  of range». Te explica los errores en español y enseña el antes y el después.
- **Explicar código**: qué hace cada línea, para qué sirve cada variable, si es eficiente, y una versión
  con comentarios.
- **Probar código**: casos de borde, propiedades, el caso más pequeño que falla y un archivo `_test.go`.
- **Ejecutar código** en una caja segura. Si el código hace algo arriesgado (leer archivos, ejecutar
  otros programas…), te avisa y te pide permiso antes.
- **Preguntas de Go**: «¿cómo uso strings.Split?», «¿para qué sirve defer?». Tiene notas en español
  de lo más común; el resto lo enseña en inglés y lo dice.
- **Aritmética exacta**: fracciones, porcentajes, mcd, números primos, factorizar.
- **Álgebra**: ecuaciones de primer y segundo grado, sistemas, inecuaciones. Enseña los pasos y
  comprueba la solución.
- **Problemas con palabras**: «el doble de un número más 3 es 11».
- **Lógica**: «si llueve el suelo se moja; llueve. ¿se moja?», silogismos, caballeros y bribones.
- **Hechos**: «recuerda que Toby es un perro» y luego «¿Toby es mamífero?».
- **Puzles**: sudoku, 8 reinas, SEND+MORE=MONEY, el acertijo de la cebra, colorear mapas.
- **Contar y probabilidad**: combinaciones, anagramas, monedas y dados, con la fórmula y contando uno
  a uno cuando se puede.
- **Secuencias**: «2, 5, 10, 17, ¿siguiente?».
- **Planificar**: jarras de agua, el lobo, la cabra y la col, torres de Hanói, laberintos.
- **Memoria**: recuerda las funciones que te hizo y las palabras que le enseñas («recuerda que capicúa
  significa palíndromo»). Puedes pedirle que las olvide.
- **Buscar en internet** (si lo dejas): «busca cómo invertir un mapa en Go». Dice de dónde sale cada cosa.
- **Seguir la conversación**: «¿y si fuera 13?», «ahora con impares», «¿por qué?», «otra forma»,
  «está mal: con [2] da 2».
- **Estudiar**: con el botón «Estudiar Go» aprende ejemplos nuevos y vuelve a intentar las preguntas
  que antes no supo resolver.

## Qué no sabe hacer

- **Entiende el español con patrones y un diccionario**, no como una persona. Las frases largas, las
  faltas de ortografía y las formas raras de decir las cosas pueden fallar. Te dice cómo lo entendió
  para que puedas corregirlo, y si no entiende algo te lo dice y te propone cómo escribirlo.
- **Solo crea programas pequeños.** Las tareas grandes (programas con mucho estado, estructuras con
  muchos métodos, mucha entrada y salida) dependen de sus recetas, de su memoria o de internet, y a
  veces termina con «No lo he conseguido».
- **Las pruebas no son demostraciones.** Las etiquetas dicen exactamente qué se comprobó. Los arreglos
  automáticos solo llegan a errores pequeños (uno o dos cambios); no rediseña tu código.
- **Internet es frágil y casi todo está en inglés.** El buscador puede cambiar o bloquear las
  búsquedas. Los trozos de código de la web a veces están incompletos y su licencia es la de cada sitio.
- **La caja segura depende de tu sistema.** Con los «user namespaces» de Linux (Debian 13 los tiene)
  el aislamiento es completo. Sin ellos es básico, y solo ejecuta código que usa paquetes seguros.
- **Puede ir lento** la primera vez que compila. Medir la velocidad de un código en un ordenador
  ocupado da resultados con ruido, por eso dice «parece».
- **Razona solo con los tipos de problemas de la lista.** Geometría, cálculo (derivadas, integrales)
  y problemas abiertos reciben «No sé plantearlo» y un ejemplo de cómo escribirlo.
- **Aprender es recordar, contar y combinar**, no entender cosas nuevas. Con el uso se hace más rápido
  y sabe más cosas de las que ya vio, pero no aprende tipos de tareas nuevos.
- **No es para charlar.** A los saludos responde con frases fijas.

## Dónde guarda sus datos

En `~/.local/share/nyx-codigo/` (o en `$XDG_DATA_HOME/nyx-codigo/` si tienes esa variable), con
permisos solo para tu usuario (0700). Con `-datos` puedes elegir otra carpeta. Dentro hay:

- `biblioteca.json`: las funciones que ha aprendido,
- `hechos.jsonl`, `reglas.jsonl`: lo que le has contado,
- `lexico.json`: las palabras nuevas,
- `aprendizaje.json`, `contraejemplos.json`, `pistas.json`: lo que aprende con el uso,
- `historial/`: lo que le preguntaste cada día,
- `pendientes.jsonl`: las preguntas que no supo resolver,
- `web/`, `estudio/`, `indice-std.json`: copias de páginas y ejemplos de Go,
- `cache/go-build/`: la caché de Go para compilar,
- `tmp/`: carpetas temporales (se vacían al cerrar),
- `registro.log`: el registro del programa (para buscar fallos),
- `instancia.json`: existe solo mientras el programa está abierto.

## Cómo empezar de cero

Cierra Nyx Código (Ctrl+C) y borra su carpeta:

```sh
rm -rf ~/.local/share/nyx-codigo
```

Si solo quieres que olvide algo concreto, no hace falta borrar nada: en la pestaña «Memoria» puedes
quitar funciones, hechos y palabras, o pedírselo: «olvida SumaPares».

## Privacidad

- **Todo pasa en tu ordenador.** El programa solo escucha en `127.0.0.1`: nadie más de tu red puede
  entrar. Cada vez que arranca crea una clave nueva, y sin esa clave no responde.
- **Nada se envía ni se sube a ningún sitio.** Tus preguntas, tu código y su memoria se quedan en la
  carpeta de datos.
- **Internet solo se usa para buscar**: cuando le pides buscar algo, para leer documentación de Go
  (go.dev, pkg.go.dev, gobyexample.com, Wikipedia) y en el modo «Estudiar». Solo descarga páginas; nunca
  ejecuta código de internet con permisos especiales.
- **Puedes quitar internet del todo** con `-sin-internet`. Todo lo demás sigue funcionando.
