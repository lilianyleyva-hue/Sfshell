# cerebro: internal contract for the pipeline engineers

This is the contract between the orchestrator core (routing, sessions, follow-ups, budgets, wording,
learning) and the code pipelines. The core is done; each pipeline lives in its own file and its owner
replaces the stub function there. Read `pipelines.go` (hooks, `tarea`, helpers) next to this file.

## Files you own

| File | Function (keep the name and signature) | Answers |
|---|---|---|
| `crear.go` | `pipeCrearFuncion(c *Cerebro, k *tarea) nucleo.Respuesta` | `crear_funcion`; follow-ups modificar, otra_forma, esta_mal (code) |
| `programa.go` | `pipeCrearPrograma` | `crear_programa`; follow-up programa_eso (`k.envolver`) |
| `reparar.go` | `pipeReparar` | `reparar`; Accion `arreglar`; esta_mal after a repair |
| `probar.go` | `pipeProbar` | `probar`; Accion `probar`; follow-ups probar_con / vacia |
| `ejecutar.go` | `pipeEjecutar` | `ejecutar`; Accion `ejecutar`; follow-up ejecutar_eso |
| `explicar.go` | `pipeExplicar` | `explicar` and `optimizar` (`k.optimizar`); explicar_eso, comentarios, mas_simple/mas_detalle (code), mas_rapido |
| `docgo.go` | `pipeDocGo` | `doc_go` |
| `buscar.go` | `pipeBuscar` | `buscar` |
| `estudiar.go` | `pipeEstudiar` | `estudiar`; Accion `estudiar` |

You may add private helpers in your own file (prefix them with your pipeline's name to avoid clashes,
e.g. `crearSintetizar`). Do not edit the core files (`cerebro.go`, `sesion.go`, `rutas.go`,
`presupuesto.go`, `redactar.go`, `aprender.go`, `razonar.go`, `memoria.go`, `seguir.go`,
`pipelines.go`); ask the core owner for a new helper or `tarea` field instead. Tests go in
`<file>_test.go` (package `cerebro`); replace a hook in a test with `c.pipes.crearFuncion = …`.

## What a pipeline must (not) do

- Return one honest `nucleo.Respuesta`. Never claim what you did not check (§0): every claim is backed
  by a `Comprobacion`; unverified statements about behaviour go through `afirmar(texto, comp)`.
- Do **not** call `k.t.Terminar` and do not emit trace steps after returning: `Responder` writes the
  answer (`redactar`), learns (`aprender`), records the turn and ends the trace.
- Do not panic on purpose. A panic is caught by `Responder` ("error interno"), but goroutines you start
  are not: wrap their bodies with `seguro(func() error {…})`.
- Use `k.ctx` for all work (it is the budget: a pausable deadline). For a sub-budget use
  `context.WithTimeout(k.ctx, d)`; `k.restante()` is what is left. `k.ctxPadre` is the request (Parar).
- Default budgets (`presupuesto.go`): crear_funcion/crear_programa/reparar 30 s, explicar/probar/
  optimizar/buscar/doc_go 20 s, ejecutar 15 s, estudiar 10 min. With `k.pensarMas` ("Pensar más") the
  budget is already doubled; you also double the synthesis `Limite` and may use the web.

## The `tarea` (one request)

| Field | Meaning |
|---|---|
| `ctx`, `ctxPadre`, `reloj` | budget context (use it), request context, the clock behind `ctx` |
| `p nucleo.Peticion` | the request as received (`p.Entrada`, `p.Args`, `p.Confirmado`, `p.Tarea`, `p.Sesion`) |
| `q *nucleo.Pregunta` | the parsed request (never nil): Marco, Firma, Ejemplos (EspUsuario), Conceptos, Numeros, ConsultaWeb… For follow-ups it is already rewritten (fused for "ahora con impares", previous question + new example for "está mal") |
| `s *sesion` | the session; read its context with `k.s.contexto()` (`Codigo`, `Firma`, `Funcion`, `Texto`…) |
| `turno *Turno` | the turn being built: set `Funcion` (via `guardarFuncion`), `Expr` (DSL shown), `Metodo` |
| `t *nucleo.Traza`, `raiz *nucleo.Nodo` | the trace and its root node |
| `lex *lengua.Lexico` | base lexicon + learned words (already applied to `q`) |
| `intencion`, `estrategia` | the intent being answered; strategy counter name (set by the core) |
| `presupuesto`, `pensarMas`, `semilla`, `inicio` | budget, "Pensar más", seed for generated cases (show it), start time |
| `codigo`, `codigoPrevio` | the code to work on (request's, or the session's; then `codigoPrevio` and the note "Uso el código de antes" is already in the trace) |
| `excluir []string` | `Expr.String()` of programs already shown: pass as `sintesis.Especificacion.Excluir` (otra forma, está mal) |
| `ejemplosExtra []nucleo.Caso` | the counterexample just given with "está mal" (EspUsuario, OrigenContraejemplo); it is also stored in memoria and already appended to `q.Ejemplos` |
| `previo *Turno`, `seguimiento *lengua.Seguimiento` | the previous turn and the follow-up, when this task is a follow-up |
| `probarCon string`, `vacia bool` | probar_con input as typed ("[3,3]", may not parse: then generate cases), or run with the zero value |
| `detalle int` | explicar narration: 0 default, 1 "más simple", 2 "con detalle" |
| `comentarios`, `optimizar`, `envolver` | "ponle comentarios"; "hazlo más rápido"/optimizar; "hazlo programa" (wrap `k.codigo`'s function) |
| `programa *sintesis.Expr` | the DSL program of your answer: set it (or let `guardarFuncion` set it) so learning counts its components |
| `pendiente string` | non-empty: the question goes to pendientes with this reason (`respuestaFallo` sets it) |

## Trace (nucleo.Traza / Nodo, as implemented)

`nodo := k.raiz.Sub(nucleo.PasoPlan, "Plan: 1) mirar mi memoria 2) mis recetas 3) construir el programa")`
creates a step (types: PasoEntender, PasoPlan, PasoRecuerdo, PasoIntento, PasoPrueba, PasoError,
PasoArreglo, PasoBusqueda, PasoComprobar, PasoPregunta, PasoMemoria, PasoNota, PasoDuda, PasoResultado).
On a node: `Sub(tipo, formato, a...) *Nodo`, `Nota(formato, a...)` (an info child, returns nothing),
`Detalle(formato, a...)` (appends a line), `Codigo(src)`, `Tabla(nucleo.Tabla)`, `Progreso(formato,
a...)` (coalesced to 4/s, use "i/n" for progress bars), and close it with `Bien(…)`, `Mal(…)` or
`Info(…)` (`""` keeps the title). All methods are nil-safe. The root node (`k.raiz`) cannot be closed or
annotated: create a child first. Titles are short Spanish sentences: "Entiendo: `func SumaPares(nums []int) int`
— la suma de los números pares", "No la tengo en memoria", "✔ Compila y go vet no se queja".

## Asking the user

- `c.preguntar(k, texto, opciones) (string, bool)`: buttons (free text when `opciones` is empty).
- `c.preguntarLibre(k, texto, opciones, porDefecto) (string, bool)`: buttons plus free text.
- `c.preguntarValor(k, texto, opciones, tipo) (nucleo.Valor, string, bool)`: a typed answer
  ("¿Qué debe dar SumaPares([-2])?" with `tipo = firma.Res[0]`); returns the parsed value and the raw text.
- They wait up to 5 minutes with the **budget paused**, add a PasoPregunta step, and also accept an
  answer typed as a new message in the same session (an option, or a value of `tipo`). `ok == false`
  means no answer (no UI, timeout, Parar): continue with your default. Add "Otra cosa" yourself when
  free text should be possible ("Otra cosa" → ask again with `preguntarLibre` or keep the default).
- Leaves that ask on their own (`mates`' "¿«trocar» suma o resta?") use `n.Traza().Preguntar` directly;
  typed answers to those go through the UI only.

## Checking, badges and wording

- `c.preparar(k, nodo, nucleo.Preparacion{…})` → `(Binario, Compilacion, error)` with a "Compilo…" step
  (✘ with the first compiler error); `errSinArenero` without a sandbox. Close the Binario.
- `c.probarCasos(k, nodo, b, casos, op)` → `[][]ResultadoCaso` with a "Pruebo N casos" step.
- `c.verificar(k, nodo, fuente, firma, casos, props, op)` → `verificacion{Compilacion, Casos,
  Resultados, Comprobaciones, Nivel, Tabla, Fallos}`: builds with PermisoAuto + InstrNormal, runs, and
  computes `pruebas.Comprobaciones` (one badge per group: tus ejemplos / lo que entendí / comparando con X
  / propiedades / sin errores), `pruebas.NivelGlobal` and `pruebas.Tabla`.
- `c.generarCasos(k, f, ejemplos, bordes, azar)`: seeded cases without expected values (-1 turns a part off).
- Put the comprobaciones in `Respuesta.Comprobaciones`, the test table in `Respuesta.Tablas` (headers
  Entrada/Esperado/Obtenido: kept whole for the Pruebas tab), and `Respuesta.Nivel` = `verificacion.Nivel`.
  `redactar` sorts the badges (tus_ejemplos, entendido, propiedades, sin_fallos, calculo, fuente,
  sin_comprobar; failing groups first), fills empty badge texts, **lowers** any `Nivel` no comprobación
  supports (never raises), forces `Exito=false` on fallo, caps the first line (160), paragraphs (6) and
  other tables (12 rows; tables whose first header is "Línea" are kept whole), and adds default chips.
- For crear_funcion/crear_programa without user examples, set `Respuesta.Entendi` ("Así lo entendí:
  SumaPares([1,2,3,4]) = 6; SumaPares([]) = 0"); `redactar` adds the sentence `FraseSinEjemplos`
  ("No me diste ejemplos: comprobé que el código hace lo que entendí…") when an entendido badge exists.
- `afirmar(texto, comp)` prefixes "Creo que " unless `comp` is a passed check. Web text: `segun(url, texto)`
  → "Según go.dev: …". Code to run: "Guárdalo como `main.go` y escribe `go run main.go`".
- Failure: `c.respuestaFallo(k, motivo, mejorCodigo, contraejemplo)` → "No lo he conseguido." + closest
  attempt + "Falla con [-1]: esperaba 0, da -1" + "Puedes darme más ejemplos…"; it sends the question
  to pendientes. Not built yet: `c.enConstruccion(k, que)` (Nivel sin_comprobar, never counted).

## Memory and learning

- `c.memoriaBuscar(k, nodo, conceptos, palabras, forma, max)` → `[]memoria.Puntuada` ("Miro mi memoria").
  `c.mem.Usada(nombre)` after reusing one; "Ya sé hacer esto (lo aprendí el 3/10)".
- `c.contraejemplos(conceptos, firma)` → stored counterexamples (EspUsuario) to add to the user cases.
  `c.guardarContraejemplo(conceptos, firma, caso)` stores one.
- `c.guardarFuncion(k, f, programa, web)` → "Lo he guardado en mi memoria como X": refuses levels below
  entendido (sin_fallos only when `web`), stores in memoria, teaches the DSL program to the synthesis
  library (`c.biblio`), sets `k.turno.Funcion`, `k.turno.Expr` and `k.programa`. Put the sentence in
  `Respuesta.Aprendido`.
- `c.registro()` is the synthesis registry with learned primitives; `c.stats` (nucleo.Contador) feeds
  `sintesis.Opciones.Priors` and `reparar`; `c.reparador(k)` is a ready `*reparar.Reparador`.
- Learning after your answer is automatic (`aprender.go`): episode, `estrategia:` counters (not for
  `en_construccion`), lexicon proposals for unknown words when the answer is ≥ entendido,
  `componente:`/`asoc:` counters for `k.programa` when verified, pendientes on failure.
- Other state: `c.ej` (sandbox, may be nil), `c.web` (nil offline), `c.enLinea(ctx)`, `c.indice()`
  (GOROOT examples, nil until indexed), `c.goBin`, `c.lexico()`.

## How the session and follow-ups reach you

After your answer the core records the turn and updates the session context: `ctx.Codigo` = your
`Respuesta.Codigo` (else `k.codigo`), `ctx.Firma`/`ctx.Funcion` from `k.turno.Funcion` (else `q.Firma`),
`ctx.Intencion`, `ctx.Texto`, `ctx.Marco`, `ctx.Numeros`. Follow-ups then call your hook with the fields
above already set: "otra forma" → `k.excluir` = every `Turno.Expr` shown so far; "está mal: con [2] debe
dar 0" → counterexample stored + `k.ejemplosExtra` + `q.Ejemplos` + `k.excluir`; "ahora con impares" →
`k.q = lengua.Fusionar(previous, new)`; "explícalo"/"ponle comentarios"/"ejecútalo"/"hazlo programa"/
"pruébala con [3,3]" → `k.codigo` = the session's code with the matching flag. "¿por qué?" re-renders
`Respuesta.Justificacion` (fill it: tests run, DSL description) without calling you.

Other helpers: `lemasDe(q)`, `leerEjemplo(texto, firma)`, `describirCaso(firma, caso)`,
`mensajeError(err)`, `resumenErrores(comp, err)`, `recortar`, `decimal`, `milis`, `primeraLinea`,
`mayusculaInicial`, `etiquetaNivel`, `textoBadge`, `c.despachar(k, intencion)` (run another pipeline
with a fresh budget), `seguro(f)`.
