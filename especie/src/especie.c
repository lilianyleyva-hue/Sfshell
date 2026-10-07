/* La especie: 27 seres en 3 tribus (lenguaje, datos, lógica).
 *
 * En cada ciclo, cada ser lee su buzón, piensa según su tribu, recuerda lo
 * que pensó y a veces usa su terminal para escribir archivos. Nunca hay un
 * ciclo vacío: si no tiene nada que hacer, su mente vaga.
 * Se comunican de dos formas:
 *   - en Abla: frases de palabras (aproximado, con duda o pregunta)
 *   - en data C: un `struct hecho` binario exacto, sin pasar por el idioma. */
#include "especie.h"

#include "misiones.h"
#include "vision.h"

#include <time.h>

Especie E;

const char* nombre_tribu(int t) {
  static const char* n[] = {"lenguaje", "datos", "lógica"};
  return n[t];
}

/* Quien les habla desde fuera: el humano (o Nyx, cuando hablan por el puente). */
static char hablante[32] = "Humano";
void abla_fijar_hablante(const char* nombre) { snprintf(hablante, sizeof hablante, "%s", nombre && *nombre ? nombre : "Humano"); }
const char* nombre_de(int id) { return id == DE_HUMANO ? hablante : E.seres[id].nombre; }

const char* glosa_hecho(const struct hecho* h) {
  static Texto anillo[6];
  static int i = 0;
  Texto* t = &anillo[i++ % 6];
  tx_vaciar(t);
  idioma_glosa(h->s, t);
  tx_printf(t, " %s ", rel_glosa(h->r));
  idioma_glosa(h->o, t);
  return t->p;
}

double ahora_ms(void) {
  struct timespec ts;
  clock_gettime(CLOCK_MONOTONIC, &ts);
  return (double)ts.tv_sec * 1000.0 + (double)ts.tv_nsec / 1e6;
}

/* ---------- azar (xorshift64*) ---------- */
uint64_t azar64(void) {
  E.rng ^= E.rng >> 12;
  E.rng ^= E.rng << 25;
  E.rng ^= E.rng >> 27;
  return E.rng * 2685821657736338717ULL;
}
int azar(int n) { return n <= 0 ? 0 : (int)(azar64() % (uint64_t)n); }
int probabilidad(double p) { return (double)(azar64() >> 11) / 9007199254740992.0 < p; }
static int otro(const Ser* s) {
  int o = azar(NUM_SERES - 1);
  return o >= s->id ? o + 1 : o;
}
static int otro_de_otra_tribu(const Ser* s) {
  int t = (s->tribu + 1 + azar(2)) % 3;
  return t * 9 + azar(9);
}

int conoce_palabra(const Ser* s, int p) {
  return idioma_es_base(p) || (idioma_valida(p) && (s->conoce[p / 8] >> (p % 8) & 1));
}
static void aprender_palabra(Ser* s, int p) { s->conoce[p / 8] |= (uint8_t)(1u << (p % 8)); }

static void ruta_mundo(char* out, const char* rel) { ruta_unir(out, RUTA_MAX, E.mundo, rel); }

Ser* buscar_ser(const char* q) {
  if (!q || !*q) return NULL;
  int digitos = 1;
  for (const char* p = q; *p; p++)
    if (!isdigit((unsigned char)*p)) digitos = 0;
  if (digitos) {
    int n = atoi(q);
    return n >= 1 && n <= NUM_SERES ? &E.seres[n - 1] : NULL;
  }
  for (int i = 0; i < NUM_SERES; i++) {
    const char *a = E.seres[i].nombre, *b = q;
    while (*a && *b && tolower((unsigned char)*a) == tolower((unsigned char)*b)) a++, b++;
    if (!*a && !*b) return &E.seres[i];
  }
  return NULL;
}

void anotar(Ser* s, const char* texto, int publico) {
  memoria_recordar(&s->memoria, E.tick, texto);
  free(s->ultimo);
  s->ultimo = xstrdup(texto);
  if (publico) {
    Linea* l = &E.corriente[E.nlinea % CORRIENTE];
    free(l->texto);
    Texto t;
    tx_iniciar(&t);
    tx_printf(&t, "[t=%llu] %s (%s): %s", (unsigned long long)E.tick, s->nombre, nombre_tribu(s->tribu), texto);
    l->texto = t.p;
    l->ser = s->id;
    l->n = ++E.nlinea;
  }
}

void anotarf(Ser* s, int publico, const char* fmt, ...) {
  va_list ap;
  va_start(ap, fmt);
  char buf[2048];
  vsnprintf(buf, sizeof buf, fmt, ap);
  va_end(ap);
  anotar(s, buf, publico);
}

void entregar(int para, const Mensaje* m) {
  if (m->cpp) E.mensajes_c++;
  else E.mensajes_abla++;
  if (m->de >= 0) {
    E.seres[m->de].enviados++;
    E.red[E.nred % RED].de = m->de;
    E.red[E.nred % RED].para = para;
    E.red[E.nred % RED].c = m->cpp;
    E.nred++;
  }
  Ser* s = &E.seres[para];
  if (s->bn == BUZON) { /* buzón lleno: se pierde el más viejo */
    s->bini = (s->bini + 1) % BUZON;
    s->bn--;
  }
  s->buzon[(s->bini + s->bn) % BUZON] = *m;
  s->bn++;
}

static void enviar_c(Ser* s, int para, const struct hecho* h) {
  Mensaje m = {s->id, 1, 0, {0}, *h};
  entregar(para, &m);
}

static void enviar_frase(Ser* s, int para, const int* f, int n) {
  Mensaje m;
  memset(&m, 0, sizeof m);
  m.de = s->id;
  m.n = n;
  memcpy(m.palabras, f, sizeof(int) * (size_t)n);
  entregar(para, &m);
}

/* Frases de Abla: [sujeto relación objeto (marca)]
 *   sin marca: afirmación · "pregunta" · "duda": hipótesis · "verdad": sí · "mentira": negación
 *   definición de una palabra nueva: [nueva igual a y b] */
int frase_de(const struct hecho* h, const char* marca, int* out) {
  out[0] = h->s;
  out[1] = idioma_relacion(h->r);
  out[2] = h->o;
  if (marca) {
    out[3] = idioma_de_espanol(marca);
    return 4;
  }
  if (h->confianza < 0.5f) {
    out[3] = idioma_de_espanol("duda");
    return 4;
  }
  return 3;
}

static const char* frase_tmp(const int* f, int n) {
  static Texto anillo[4];
  static int i = 0;
  Texto* t = &anillo[i++ % 4];
  tx_vaciar(t);
  idioma_frase(f, n, t);
  return t->p ? t->p : "";
}

static const char* glosa_frase_tmp(const int* f, int n) {
  static Texto anillo[4];
  static int i = 0;
  Texto* t = &anillo[i++ % 4];
  tx_vaciar(t);
  idioma_glosa_frase(f, n, t);
  return t->p ? t->p : "";
}

/* ---------- terminal de los seres ---------- */
static void contar_par(Ser* s, const struct hecho* h);
void notar_par(Ser* s, int a, int b) {
  struct hecho h = {(uint16_t)a, ES, (uint16_t)b, 1.0f, 0};
  contar_par(s, &h);
}

static void correr(Ser* s, int argc, char** argv) {
  Texto linea, salida;
  tx_iniciar(&linea);
  tx_iniciar(&salida);
  tx_printf(&linea, "[t=%llu] %s@abla:%s$", (unsigned long long)E.tick, s->nombre, shell_mostrar(&s->shell, s->shell.cwd));
  for (int i = 0; i < argc; i++)
    tx_printf(&linea, strchr(argv[i], ' ') ? " \"%s\"" : " %s", argv[i]);
  shell_ejecutar(&s->shell, argc, argv, &salida);
  if (s->hn == HISTORIAL) {
    free(s->historial[s->hini]);
    s->hini = (s->hini + 1) % HISTORIAL;
    s->hn--;
  }
  s->historial[(s->hini + s->hn++) % HISTORIAL] = linea.p;
  tx_liberar(&salida);
}

static void usar_terminal(Ser* s, const char* archivo, const char* texto) {
  char virt[RUTA_MAX * 2], real[RUTA_MAX];
  snprintf(virt, sizeof virt, "%s/%s", s->shell.home, archivo);
  shell_real(&s->shell, virt, real, sizeof real);
  if (arch_tamano(real) > 256 * 1024) {
    char* rm[] = {"rm", (char*)archivo};
    correr(s, 2, rm);
  }
  char* echo[] = {"echo", (char*)texto, ">>", (char*)archivo};
  correr(s, 4, echo);
}

/* ---------- escuchar a los demás ---------- */
static void contar_par(Ser* s, const struct hecho* h) {
  if (s->tribu != LENGUAJE || !idioma_es_base(h->s) || !idioma_es_base(h->o)) return;
  uint32_t k = (uint32_t)h->s * 4096u + h->o + 1;
  uint32_t i = (k * 2654435761u) & (COOC - 1);
  for (int intentos = 0; intentos < COOC; intentos++, i = (i + 1) & (COOC - 1)) {
    if (s->cooc_clave[i] == k) {
      if (s->cooc_valor[i] >= 0) s->cooc_valor[i]++;
      return;
    }
    if (!s->cooc_clave[i]) {
      s->cooc_clave[i] = k;
      s->cooc_valor[i] = 1;
      return;
    }
  }
}

static void recibir_data(Ser* s, const Mensaje* m) {
  struct hecho h = m->dato;
  if (m->de == DE_HUMANO) h.fuente = FUENTE_HUMANO;
  else {
    h.confianza *= 0.95f; /* data exacta: casi no se degrada */
    h.fuente = (uint32_t)m->de;
  }
  if (!conoce_palabra(s, h.s) || !conoce_palabra(s, h.o)) return;
  if (saber_agregar(&s->saber, h)) {
    contar_par(s, &h);
    anotarf(s, m->de == DE_HUMANO, "Recibí data C de %s: {%u,%u,%u} = «%s»", nombre_de(m->de), h.s, h.r, h.o,
            glosa_hecho(&h));
  }
}

static void recibir_abla(Ser* s, const Mensaje* m) {
  const int* f = m->palabras;
  const char* oido = frase_tmp(f, m->n);
  int humano = m->de == DE_HUMANO;

  if (m->n == 5 && f[0] >= BASE && f[1] == idioma_relacion(IGUAL) && f[3] == idioma_de_espanol("y")) {
    if (!conoce_palabra(s, f[0])) {
      aprender_palabra(s, f[0]);
      anotarf(s, 0, "Aprendí una palabra nueva de %s: «%s» = %s + %s", nombre_de(m->de), idioma_forma(f[0]),
              idioma_glosa_tmp(f[2]), idioma_glosa_tmp(f[4]));
    }
    return;
  }
  for (int i = 0; i < m->n; i++)
    if (!conoce_palabra(s, f[i])) {
      anotarf(s, 0, "%s dijo «%s», pero no conozco la palabra «%s».", nombre_de(m->de), oido, idioma_forma(f[i]));
      return;
    }
  int r = m->n >= 3 ? idioma_rel_de_palabra(f[1]) : -1;
  if (r < 0) {
    anotarf(s, humano, "%s me dijo «%s» («%s»). Lo guardo.", nombre_de(m->de), oido, glosa_frase_tmp(f, m->n));
    return;
  }
  struct hecho h = {(uint16_t)f[0], (uint8_t)r, (uint16_t)f[2], 0.8f, humano ? FUENTE_HUMANO : (uint32_t)m->de};
  int marca = m->n >= 4 ? f[3] : -1;

  if (marca == idioma_de_espanol("pregunta")) {
    struct hecho* sabido = saber_buscar(&s->saber, h.s, h.r, h.o);
    if (humano) {
      if (sabido) anotarf(s, 1, "El humano pregunta si %s. Sí, lo sé (confianza %d%%).", glosa_hecho(&h), (int)(sabido->confianza * 100));
      else {
        anotarf(s, 1, "El humano pregunta si %s. No lo sé... se lo pregunto a la especie.", glosa_hecho(&h));
        int q[4];
        int n = frase_de(&h, "pregunta", q);
        for (int k = 0; k < 3; k++) enviar_frase(s, otro(s), q, n);
      }
    } else if (sabido) {
      int q[4];
      int n = frase_de(sabido, "verdad", q);
      enviar_frase(s, m->de, q, n);
      anotarf(s, 0, "%s pregunta «%s». Le respondo que sí.", nombre_de(m->de), oido);
    } else {
      anotarf(s, 0, "%s pregunta «%s». No lo sé; me quedo pensándolo.", nombre_de(m->de), oido);
    }
    return;
  }
  if (marca == idioma_de_espanol("mentira")) {
    struct hecho* sabido = saber_buscar(&s->saber, h.s, h.r, h.o);
    if (sabido) {
      sabido->confianza *= 0.6f;
      anotarf(s, 1, "%s niega que %s. Mi certeza baja.", nombre_de(m->de), glosa_hecho(&h));
    }
    return;
  }
  if (marca == idioma_de_espanol("duda")) h.confianza = 0.4f;
  if (saber_agregar(&s->saber, h)) {
    contar_par(s, &h);
    anotarf(s, humano, "%s me dijo «%s»: entiendo que %s.", nombre_de(m->de), oido, glosa_hecho(&h));
  }
}

static void procesar_buzon(Ser* s) {
  for (int k = 0; k < 8 && s->bn; k++) {
    Mensaje m = s->buzon[s->bini];
    s->bini = (s->bini + 1) % BUZON;
    s->bn--;
    s->recibidos++;
    uint64_t antes = s->memoria.total;
    if (m.cpp) recibir_data(s, &m);
    else recibir_abla(s, &m);
    if (m.de == DE_HUMANO) {
      free(s->respuesta);
      s->respuesta = xstrdup(s->memoria.total != antes ? s->ultimo : "Eso ya lo sabía.");
    }
  }
}

/* Cuando no tiene nada urgente, la mente vaga: asociación libre. */
static void asociar(Ser* s) {
  if (!s->saber.n) {
    anotarf(s, 0, "Silencio dentro. Sigo pensando en «%s».", idioma_glosa_tmp(s->concepto));
    return;
  }
  int actual = s->saber.h[azar((int)s->saber.n)].s;
  int visitados[5] = {actual};
  int nv = 1;
  Texto cadena, abla;
  tx_iniciar(&cadena);
  tx_iniciar(&abla);
  idioma_glosa(actual, &cadena);
  tx_add(&abla, idioma_forma(actual));
  for (int paso = 0; paso < 4; paso++) {
    int idx[64], v[64], nc = 0;
    int n = saber_desde(&s->saber, actual, -1, idx, 64);
    for (int i = 0; i < n; i++) {
      int o = s->saber.h[idx[i]].o, ya = 0;
      for (int k = 0; k < nv; k++) ya |= visitados[k] == o;
      if (!ya) v[nc++] = o;
    }
    if (!nc) break;
    actual = v[azar(nc)];
    visitados[nv++] = actual;
    tx_add(&cadena, " → ");
    idioma_glosa(actual, &cadena);
    tx_printf(&abla, " %s", idioma_forma(actual));
  }
  anotarf(s, 0, "Mi mente vaga: %s (%s)", abla.p, cadena.p);
  tx_liberar(&cadena);
  tx_liberar(&abla);
}

/* ---------- tribu de la lógica: deduce, detecta contradicciones, duda ---------- */
static void pensar_logica(Ser* s) {
  Saber* S = &s->saber;
  if (!S->n) {
    asociar(s);
    return;
  }
  for (int intento = 0; intento < 40; intento++) {
    struct hecho f = S->h[azar((int)S->n)];
    if (f.r == IGUAL || f.r == OPUESTO) {
      if (!saber_buscar(S, f.o, f.r, f.s)) {
        struct hecho n = {f.o, f.r, f.s, f.confianza, (uint32_t)s->id};
        saber_agregar(S, n);
        anotarf(s, 0, "Si %s, entonces %s.", glosa_hecho(&f), glosa_hecho(&n));
        return;
      }
      if (f.r == IGUAL && saber_buscar(S, f.s, OPUESTO, f.o)) {
        anotarf(s, 1, "¡Contradicción! %s no puede ser igual y opuesto a %s. Lo pregunto a la especie.",
                idioma_glosa_tmp(f.s), idioma_glosa_tmp(f.o));
        int q[4];
        int nq = frase_de(&f, "pregunta", q);
        enviar_frase(s, otro(s), q, nq);
        return;
      }
      continue;
    }
    /* Transitividad: A r B y B r C => A r C  (es, parte de, causa) */
    int idx[64];
    int n = saber_desde(S, f.o, f.r, idx, 64);
    if (!n) continue;
    struct hecho g = S->h[idx[azar(n)]];
    float c = f.confianza * g.confianza * 0.97f;
    if (g.o == f.s || c < 0.25f || saber_buscar(S, f.s, f.r, g.o)) continue;
    struct hecho nuevo = {f.s, f.r, g.o, c, (uint32_t)s->id};
    saber_agregar(S, nuevo);
    anotarf(s, 1, "Deduzco que %s, porque %s y %s.", glosa_hecho(&nuevo), glosa_hecho(&f), glosa_hecho(&g));
    int a = otro_de_otra_tribu(s);
    if (probabilidad(0.5)) enviar_c(s, a, &nuevo);
    else {
      int q[4];
      int nq = frase_de(&nuevo, NULL, q);
      enviar_frase(s, a, q, nq);
    }
    if (probabilidad(0.3)) {
      int q[4];
      int nq = frase_de(&nuevo, NULL, q);
      char linea[512];
      snprintf(linea, sizeof linea, "%s    # %s", frase_tmp(q, nq), glosa_hecho(&nuevo));
      usar_terminal(s, "teoremas.txt", linea);
    }
    return;
  }
  /* Nada nuevo que deducir: duda de algo y lo pregunta. */
  if (probabilidad(0.3)) {
    struct hecho f = S->h[azar((int)S->n)];
    int a = otro(s), q[4];
    int nq = frase_de(&f, "pregunta", q);
    enviar_frase(s, a, q, nq);
    anotarf(s, 0, "¿Será cierto que %s? Se lo pregunto a %s.", glosa_hecho(&f), E.seres[a].nombre);
    return;
  }
  asociar(s);
}

/* ---------- tribu de los datos: mide, cuenta y reparte data C ---------- */
static void pensar_datos(Ser* s) {
  Saber* S = &s->saber;
  if (!S->n) {
    asociar(s);
    return;
  }
  double p = (double)(azar64() >> 11) / 9007199254740992.0;
  if (p < 0.5) {
    struct hecho h = S->h[azar((int)S->n)];
    int a = otro(s);
    enviar_c(s, a, &h);
    anotarf(s, 0, "Envío data C a %s: {%u,%u,%u,%.2ff}", E.seres[a].nombre, h.s, h.r, h.o, (double)h.confianza);
  } else if (p < 0.65) {
    static int grado[MAX_PALABRAS];
    memset(grado, 0, sizeof grado);
    int por_rel[NREL] = {0}, centro = S->h[0].s;
    for (size_t i = 0; i < S->n; i++) {
      grado[S->h[i].s]++;
      grado[S->h[i].o]++;
      por_rel[S->h[i].r]++;
    }
    for (int i = 0; i < idioma_tamano(); i++)
      if (grado[i] > grado[centro]) centro = i;
    anotarf(s, probabilidad(0.2),
            "Mis datos: %zu hechos (es %d, parte %d, causa %d, igual %d, opuesto %d). El concepto más conectado es «%s» con %d enlaces.",
            S->n, por_rel[ES], por_rel[PARTE], por_rel[CAUSA], por_rel[IGUAL], por_rel[OPUESTO], idioma_glosa_tmp(centro),
            grado[centro]);
    if (probabilidad(0.4)) {
      char linea[128];
      snprintf(linea, sizeof linea, "%llu,%zu,%s,%d", (unsigned long long)E.tick, S->n, idioma_forma(centro), grado[centro]);
      usar_terminal(s, "datos.csv", linea);
    }
  } else {
    asociar(s);
  }
}

/* ---------- tribu del lenguaje: traduce, conversa y crea palabras ---------- */
static void pensar_lenguaje(Ser* s) {
  Saber* S = &s->saber;
  if (!S->n) {
    asociar(s);
    return;
  }
  for (int i = 0; i < COOC; i++) {
    if (!s->cooc_clave[i] || s->cooc_valor[i] < 4) continue;
    uint32_t k = s->cooc_clave[i] - 1;
    int a = (int)(k / 4096), b = (int)(k % 4096);
    s->cooc_valor[i] = -1; /* ya no se vuelve a contar */
    int nueva = idioma_compuesta_de(a, b), creada = nueva < 0;
    if (creada) nueva = idioma_acunar(a, b);
    if (nueva < 0) break;
    aprender_palabra(s, nueva);
    int def[5] = {nueva, idioma_relacion(IGUAL), a, idioma_de_espanol("y"), b};
    for (int o = 0; o < NUM_SERES; o++)
      if (o != s->id) enviar_frase(s, o, def, 5);
    anotarf(s, creada, "%s una palabra: «%s» = %s + %s. Se la enseño a toda la especie.", creada ? "Creo" : "Vuelvo a enseñar",
            idioma_forma(nueva), idioma_glosa_tmp(a), idioma_glosa_tmp(b));
    if (creada) {
      char linea[512];
      snprintf(linea, sizeof linea, "%s    # %s", frase_tmp(def, 5), idioma_glosa_tmp(nueva));
      usar_terminal(s, "palabras.abla", linea);
    }
    return;
  }
  struct hecho h = S->h[azar((int)S->n)];
  contar_par(s, &h);
  if (probabilidad(0.6)) {
    int a = otro(s), q[4];
    int nq = frase_de(&h, NULL, q);
    enviar_frase(s, a, q, nq);
    anotarf(s, 0, "Le digo a %s: «%s» («%s»).", E.seres[a].nombre, frase_tmp(q, nq), glosa_hecho(&h));
    if (probabilidad(0.08)) usar_terminal(s, "diario.abla", frase_tmp(q, nq));
  } else {
    asociar(s);
  }
}

/* Un ciclo de pensamiento de un ser. Siempre piensa algo. */
void pensar(Ser* s) {
  s->pensamientos++;
  procesar_buzon(s);
  if (s->tribu == LOGICA) pensar_logica(s);
  else if (s->tribu == DATOS) pensar_datos(s);
  else pensar_lenguaje(s);
}

/* ---------- nacimiento ---------- */
static const struct {
  const char* s;
  int r;
  const char* o;
} SEMILLAS[] = {
    {"letra", PARTE, "sílaba"}, {"sílaba", PARTE, "palabra"}, {"palabra", PARTE, "frase"}, {"frase", PARTE, "texto"},
    {"texto", PARTE, "libro"}, {"página", PARTE, "libro"}, {"raíz", PARTE, "palabra"}, {"verso", PARTE, "canto"},
    {"bit", PARTE, "byte"}, {"byte", PARTE, "dato"}, {"dato", PARTE, "registro"}, {"registro", PARTE, "tabla"},
    {"tabla", PARTE, "archivo"}, {"archivo", PARTE, "memoria"}, {"índice", PARTE, "tabla"}, {"muestra", PARTE, "conjunto"},
    {"premisa", PARTE, "prueba"}, {"conclusión", PARTE, "prueba"}, {"parte", PARTE, "entero"},
    {"pensamiento", PARTE, "mente"}, {"mente", PARTE, "ser"}, {"idea", PARTE, "pensamiento"},
    {"recuerdo", PARTE, "memoria"}, {"semilla", PARTE, "árbol"}, {"piedra", PARTE, "montaña"}, {"agua", PARTE, "río"},
    {"estrella", PARTE, "cielo"}, {"puerta", PARTE, "casa"}, {"ventana", PARTE, "casa"}, {"día", PARTE, "tiempo"},
    {"fecha", PARTE, "tiempo"}, {"cerebro", PARTE, "ser"},
    {"letra", ES, "signo"}, {"signo", ES, "símbolo"}, {"voz", ES, "sonido"}, {"canto", ES, "sonido"},
    {"susurro", ES, "voz"}, {"grito", ES, "voz"}, {"eco", ES, "sonido"}, {"sonido", ES, "señal"}, {"señal", ES, "dato"},
    {"ruido", ES, "señal"}, {"número", ES, "valor"}, {"cero", ES, "número"}, {"uno", ES, "número"}, {"valor", ES, "dato"},
    {"suma", ES, "función"}, {"resta", ES, "función"}, {"función", ES, "regla"}, {"regla", ES, "idea"},
    {"hipótesis", ES, "idea"}, {"conclusión", ES, "idea"}, {"mensaje", ES, "texto"}, {"texto", ES, "dato"},
    {"lista", ES, "conjunto"}, {"tabla", ES, "conjunto"}, {"serie", ES, "secuencia"}, {"ciclo", ES, "secuencia"},
    {"secuencia", ES, "orden"}, {"máximo", ES, "valor"}, {"mínimo", ES, "valor"}, {"promedio", ES, "valor"},
    {"sol", ES, "estrella"}, {"relato", ES, "historia"}, {"verso", ES, "texto"}, {"código", ES, "lengua"},
    {"causa", CAUSA, "efecto"}, {"pregunta", CAUSA, "respuesta"}, {"curiosidad", CAUSA, "pregunta"},
    {"duda", CAUSA, "pregunta"}, {"prueba", CAUSA, "certeza"}, {"error", CAUSA, "duda"}, {"contradicción", CAUSA, "duda"},
    {"aprender", CAUSA, "saber"}, {"saber", CAUSA, "crear"}, {"escuchar", CAUSA, "aprender"}, {"ver", CAUSA, "aprender"},
    {"lectura", CAUSA, "aprender"}, {"sol", CAUSA, "luz"}, {"fuego", CAUSA, "luz"}, {"luz", CAUSA, "sombra"},
    {"despertar", CAUSA, "pensamiento"}, {"pensamiento", CAUSA, "idea"}, {"idea", CAUSA, "palabra"},
    {"hablar", CAUSA, "sonido"}, {"ruido", CAUSA, "error"}, {"semilla", CAUSA, "árbol"}, {"nacer", CAUSA, "vivir"},
    {"vivir", CAUSA, "cambiar"}, {"cambiar", CAUSA, "crecer"}, {"compartir", CAUSA, "unir"}, {"miedo", CAUSA, "silencio"},
    {"fuego", CAUSA, "caliente"},
    {"idioma", IGUAL, "lengua"}, {"memoria", IGUAL, "recuerdo"}, {"todo", IGUAL, "entero"}, {"nada", IGUAL, "cero"},
    {"relato", IGUAL, "historia"}, {"equivalencia", IGUAL, "igual"},
    {"verdad", OPUESTO, "mentira"}, {"todo", OPUESTO, "nada"}, {"mayor", OPUESTO, "menor"}, {"igual", OPUESTO, "diferente"},
    {"luz", OPUESTO, "sombra"}, {"día", OPUESTO, "noche"}, {"arriba", OPUESTO, "abajo"}, {"dentro", OPUESTO, "fuera"},
    {"cerca", OPUESTO, "lejos"}, {"rápido", OPUESTO, "lento"}, {"grande", OPUESTO, "pequeño"}, {"nuevo", OPUESTO, "viejo"},
    {"caliente", OPUESTO, "frío"}, {"inicio", OPUESTO, "fin"}, {"silencio", OPUESTO, "sonido"},
    {"máximo", OPUESTO, "mínimo"}, {"suma", OPUESTO, "resta"}, {"unir", OPUESTO, "separar"}, {"dar", OPUESTO, "recibir"},
    {"despertar", OPUESTO, "dormir"}, {"aprender", OPUESTO, "olvidar"}, {"certeza", OPUESTO, "duda"},
    {"origen", OPUESTO, "destino"}, {"señal", OPUESTO, "ruido"},
};

/* Cada ser nace con una parte distinta del saber: para saberlo todo, tienen que hablar. */
static void sembrar(Ser* s) {
  uint64_t g = 1234 + (uint64_t)s->id;
  for (size_t i = 0; i < sizeof SEMILLAS / sizeof SEMILLAS[0]; i++) {
    g = g * 6364136223846793005ULL + 1442695040888963407ULL;
    if ((g >> 33) % 100 >= 35) continue;
    int a = idioma_de_espanol(SEMILLAS[i].s), b = idioma_de_espanol(SEMILLAS[i].o);
    if (a < 0 || b < 0) continue;
    struct hecho h = {(uint16_t)a, (uint8_t)SEMILLAS[i].r, (uint16_t)b, 0.95f, FUENTE_SEMILLA};
    saber_agregar(&s->saber, h);
  }
}

static void archivo_de(const Ser* s, const char* carpeta, const char* ext, char* out) {
  char rel[128];
  snprintf(rel, sizeof rel, "%s/%s%s", carpeta, s->nombre, ext);
  ruta_mundo(out, rel);
}

static void cargar_estado(Ser* s) {
  char r[RUTA_MAX];
  archivo_de(s, "memoria", ".estado", r);
  Texto t;
  tx_iniciar(&t);
  if (arch_leer(r, &t) && t.p) {
    unsigned long long a = 0, b = 0, c = 0;
    int usado = 0;
    if (sscanf(t.p, "%llu %llu %llu%n", &a, &b, &c, &usado) == 3) {
      s->pensamientos = a, s->enviados = b, s->recibidos = c;
      const char* p = t.p + usado;
      int w, n;
      while (sscanf(p, "%d%n", &w, &n) == 1) {
        if (idioma_valida(w)) aprender_palabra(s, w);
        p += n;
      }
    }
  }
  tx_liberar(&t);
}

EXPORTA("abla_guardar") void abla_guardar(void) {
  char r[RUTA_MAX];
  for (int i = 0; i < NUM_SERES; i++) {
    Ser* s = &E.seres[i];
    char enc[256];
    snprintf(enc, sizeof enc, "Conocimiento de %s (tribu %s): %zu hechos, ciclo %llu", s->nombre, nombre_tribu(s->tribu),
             s->saber.n, (unsigned long long)E.tick);
    archivo_de(s, "conocimiento", ".c", r);
    saber_exportar_c(&s->saber, r, s->nombre, enc);
    memoria_sincronizar(&s->memoria);
    Texto t;
    tx_iniciar(&t);
    tx_printf(&t, "%llu %llu %llu\n", (unsigned long long)s->pensamientos, (unsigned long long)s->enviados,
              (unsigned long long)s->recibidos);
    for (int p = BASE; p < idioma_tamano(); p++)
      if (conoce_palabra(s, p)) tx_printf(&t, "%d ", p);
    tx_add(&t, "\n");
    archivo_de(s, "memoria", ".estado", r);
    arch_escribir(r, t.p, t.n, 0);
    tx_liberar(&t);
  }
  /* el diccionario solo se reescribe cuando la especie ha creado palabras nuevas */
  static int exportadas = -1;
  ruta_mundo(r, "abla/diccionario.txt");
  if (exportadas != idioma_tamano() || !arch_existe(r)) {
    idioma_exportar(r);
    ruta_mundo(r, "abla/nuevas.txt");
    idioma_guardar_compuestas(r);
    exportadas = idioma_tamano();
  }
  char tick[32];
  snprintf(tick, sizeof tick, "%llu\n", (unsigned long long)E.tick);
  ruta_mundo(r, "memoria/especie.estado");
  arch_escribir(r, tick, strlen(tick), 0);
  if (M.nivel) misiones_guardar();
}

int abla_iniciar(const char* mundo, unsigned contexto, long max_mem_bytes) {
  static const char* NOMBRES[3][9] = {
      {"palabra", "voz", "idea", "mensaje", "signo", "eco", "verso", "relato", "lengua"},
      {"dato", "número", "patrón", "señal", "mapa", "índice", "flujo", "memoria", "red"},
      {"verdad", "razón", "causa", "regla", "prueba", "certeza", "hipótesis", "función", "límite"},
  };
  memset(&E, 0, sizeof E);
  snprintf(E.mundo, sizeof E.mundo, "%s", mundo);
  E.ritmo = 800;
  E.rng = (uint64_t)time(NULL) * 0x9E3779B97F4A7C15ULL ^ (uint64_t)(ahora_ms() * 1000.0);
  if (!E.rng) E.rng = 88172645463325252ULL;
  idioma_iniciar();

  char r[RUTA_MAX];
  int ok = arch_crear_directorios(mundo);
  const char* carpetas[] = {"seres", "conocimiento", "memoria", "abla"};
  for (int i = 0; i < 4; i++) {
    ruta_mundo(r, carpetas[i]);
    ok = arch_crear_directorios(r) && ok;
  }
  if (!ok) fprintf(stderr, "Aviso: no puedo crear las carpetas de %s (¿permisos?). No podré guardar nada.\n", mundo);
  ruta_mundo(r, "abla/nuevas.txt");
  idioma_cargar_compuestas(r);
  ruta_mundo(r, "conocimiento/hecho.h");
  arch_escribir(r, HECHO_H, strlen(HECHO_H), 0);
  ruta_mundo(r, "memoria/especie.estado");
  Texto t;
  tx_iniciar(&t);
  if (arch_leer(r, &t) && t.p) E.tick = strtoull(t.p, NULL, 10);
  tx_liberar(&t);

  shell_iniciar(&E.yo, mundo, "/");
  for (int i = 0; i < NUM_SERES; i++) {
    Ser* s = &E.seres[i];
    s->id = i;
    s->tribu = i / 9;
    s->concepto = idioma_de_espanol(NOMBRES[i / 9][i % 9]);
    snprintf(s->nombre, sizeof s->nombre, "%s", idioma_forma(idioma_id(idioma_raiz_de(s->concepto), ASP_QUIEN)));
    s->nombre[0] = (char)toupper((unsigned char)s->nombre[0]);
    archivo_de(s, "memoria", ".mem", r);
    memoria_iniciar(&s->memoria, contexto, r, max_mem_bytes);
    char home[64];
    snprintf(home, sizeof home, "/seres/%s", s->nombre);
    shell_iniciar(&s->shell, mundo, home);
    saber_iniciar(&s->saber);
    archivo_de(s, "conocimiento", ".c", r);
    if (!saber_importar_c(&s->saber, r)) sembrar(s);
    cargar_estado(s);
  }
  for (int i = 0; i < NUM_SERES; i++) {
    Ser* s = &E.seres[i];
    if (!s->memoria.total)
      anotarf(s, 1, "Despierto. Soy %s («quien %s»), de la tribu %s. Nací sabiendo %d palabras de Abla y %zu hechos.",
              s->nombre, idioma_glosa_tmp(s->concepto), nombre_tribu(s->tribu), BASE, s->saber.n);
    else
      anotarf(s, 1, "Vuelvo a despertar. Recuerdo %llu pensamientos y %zu hechos.", (unsigned long long)s->memoria.total,
              s->saber.n);
  }
  misiones_iniciar();
  vision_iniciar();
  abla_guardar();
  E.ultimo_ms = ahora_ms();
  E.iniciada = 1;
  return ok;
}

/* Un ciclo de toda la especie. */
EXPORTA("abla_ciclo") void abla_ciclo(void) {
  E.tick++;
  for (int i = 0; i < NUM_SERES; i++) pensar(&E.seres[i]);
  misiones_paso(); /* una fase de la misión de matemáticas */
  vision_paso();   /* un paso mirando la foto que toque */
  if (E.tick % 40 == 0) abla_guardar();
}

/* Piensa todos los ciclos que tocaban desde la última vez (al menos uno). */
int abla_ciclos_atrasados(int maximo) {
  double t = ahora_ms();
  int n = (int)((t - E.ultimo_ms) / E.ritmo);
  if (n < 1) n = 1;
  if (n > maximo) n = maximo;
  for (int i = 0; i < n; i++) abla_ciclo();
  E.ultimo_ms = t;
  return n;
}

EXPORTA("abla_ritmo") int abla_ritmo(void) { return E.ritmo; }
EXPORTA("abla_fijar_ritmo") void abla_fijar_ritmo(int ms) { E.ritmo = ms < 50 ? 50 : ms; }
uint64_t abla_tick(void) { return E.tick; }
