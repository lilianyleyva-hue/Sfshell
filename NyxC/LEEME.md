# Nyx — las 18 IAs, en C (varios archivos)

Para **C Free** o cualquier compilador de C. Todo es C99 estándar.

## Cómo correrlo

- **C Free:** un proyecto con **todos los `.c`** de esta carpeta (y los `.h` al lado). El programa empieza en `main.c`.
- **Terminal:** `cc *.c -o nyx -lm && ./nyx`
- ¿Tu compilador solo acepta un archivo? Usa `nyx_unico.c` (es lo mismo, junto).

## Archivos

| archivo | qué hace |
|---|---|
| `main.c` | la interfaz: aquí empieza el programa |
| `nyx.h` | tipos y funciones que comparten todos |
| `base.c` | azar, la infancia, cómo es cada una de las 18 |
| `resh.c` + `resh.h` | la lengua Resh (3144 palabras) |
| `texto.c` | palabras y firmas |
| `mente.c` | una mente: semiones, resonancia, veredicto, frases propias |
| `frases.c` | **nuevo:** memoria de frases enteras |
| `consejo.c` | las 18 juntas: imaginación, deliberación, tu opinión |
| `memoria.c` | guardar y cargar (`nyx_memoria.txt`) |
| `libro.txt` | un librito para enseñarles: `lee libro.txt` |

## Lo nuevo: memoria de frases

Ahora recuerdan las frases enteras que leen. Para responder, la mente que
habla busca las frases que contienen su idea y elige la que mejor encaja con
tu pregunta. Si ninguna encaja, arma una propia. La respuesta dice cuál fue:
"lo recordó de lo que leyó" o "frase propia".

```
qué es el mar                → el mar es grande profundo y azul
para qué sirve la paciencia  → la paciencia ayuda a resolver los problemas difíciles
@curiosidad el miedo         → la curiosidad abre puertas que el miedo cierra
```

`mal` también castiga la frase: la próxima vez gana otra.
Cuanto más les enseñes (`enseña …`, `lee archivo.txt`), mejor responden.

## Órdenes

```
<pregunta>              las 18 deliberan
@<mente> <texto>        hablas con una
bien · mal              tu opinión
vivo 30                 verlas vivir en tiempo real
conversa narrativa empatia 8
enseña <texto> · lee <archivo>
resh <palabra> · mentes · mente <nombre>
ayuda · salir
```
