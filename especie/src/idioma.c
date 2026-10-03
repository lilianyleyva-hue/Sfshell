/* Abla: el idioma con el que nace la especie.
 *
 * 200 raíces (conceptos) x 20 aspectos = 4000 palabras. Se generan de forma
 * determinista con un generador propio (splitmix64), así que cada ser las
 * conoce desde que despierta y el idioma es idéntico en cualquier máquina.
 * La tribu del lenguaje puede acuñar palabras compuestas nuevas (terminan en "sh").
 *
 * Forma de una palabra: raíz de dos sílabas (CV CV) + sufijo de aspecto (CV). */
#include "comun.h"

static const char* RAIZ[RAICES] = {
    /* lenguaje */
    "palabra", "frase", "nombre", "verbo", "sonido", "letra", "significado", "idioma", "voz", "pregunta",
    "respuesta", "historia", "signo", "símbolo", "idea", "mensaje", "texto", "gramática", "raíz", "sílaba",
    "silencio", "canto", "diálogo", "metáfora", "traducción", "lectura", "escritura", "gesto", "código",
    "relato", "verso", "eco", "acento", "rima", "libro", "página", "tinta", "susurro", "grito", "lengua",
    /* datos */
    "dato", "número", "cero", "uno", "cantidad", "lista", "tabla", "archivo", "bit", "byte", "patrón",
    "señal", "ruido", "medida", "cuenta", "suma", "resta", "orden", "serie", "conjunto", "mapa", "índice",
    "registro", "flujo", "memoria", "copia", "origen", "destino", "valor", "tipo", "tamaño", "tiempo",
    "fecha", "frecuencia", "promedio", "máximo", "mínimo", "muestra", "error", "red",
    /* lógica */
    "verdad", "mentira", "razón", "causa", "efecto", "regla", "prueba", "si", "entonces", "y", "o", "no",
    "todo", "nada", "algo", "igual", "diferente", "mayor", "menor", "parte", "entero", "clase", "ejemplo",
    "límite", "infinito", "paradoja", "duda", "certeza", "hipótesis", "conclusión", "premisa",
    "contradicción", "implicación", "equivalencia", "secuencia", "ciclo", "función", "variable",
    "constante", "problema",
    /* mente */
    "yo", "tú", "nosotros", "ellos", "ser", "mente", "cerebro", "pensamiento", "recuerdo", "sueño", "deseo",
    "miedo", "alegría", "curiosidad", "atención", "despertar", "dormir", "aprender", "olvidar", "saber",
    "creer", "buscar", "encontrar", "crear", "dar", "recibir", "hablar", "escuchar", "ver", "sentir",
    "vivir", "nacer", "cambiar", "crecer", "unir", "separar", "compartir", "preguntar", "responder",
    "imaginar",
    /* mundo */
    "luz", "sombra", "agua", "fuego", "tierra", "aire", "cielo", "sol", "luna", "estrella", "árbol",
    "semilla", "río", "mar", "montaña", "camino", "casa", "puerta", "ventana", "piedra", "hierro", "vidrio",
    "día", "noche", "inicio", "fin", "arriba", "abajo", "dentro", "fuera", "cerca", "lejos", "rápido",
    "lento", "grande", "pequeño", "nuevo", "viejo", "caliente", "frío",
};

static const char* ASPECTO[ASPECTOS] = {
    "",      "muchos",    "negado", "pregunta", "pasado", "futuro", "que causa",
    "resultado", "muy",   "poco",   "comienzo", "final",  "trozo",  "totalidad",
    "como",  "contrario", "quien",  "lugar",    "acto",   "cualidad",
};

static const char* REL_NOMBRE[NREL] = {"es", "parte", "causa", "igual", "opuesto"};
static const char* REL_GLOSA[NREL] = {"es", "es parte de", "causa", "se parece a", "es lo opuesto de"};
static const char* REL_RAIZ[NREL] = {"ser", "parte", "causa", "igual", "diferente"};

const char* idioma_raiz(int r) { return RAIZ[r]; }
const char* idioma_aspecto(int a) { return ASPECTO[a]; }
const char* rel_nombre(int r) { return REL_NOMBRE[r]; }
const char* rel_glosa(int r) { return REL_GLOSA[r]; }
int rel_de_nombre(const char* s) {
  for (int r = 0; r < NREL; r++)
    if (strcmp(s, REL_NOMBRE[r]) == 0) return r;
  return -1;
}

static char formas[MAX_PALABRAS][FORMA_MAX];
static int npalabras = 0;
static int comp_a[MAX_PALABRAS - BASE], comp_b[MAX_PALABRAS - BASE];

/* Tabla hash forma → id (abierta, potencia de 2). Guarda id+1; 0 = vacío. */
#define TABLA 32768
static int tabla[TABLA];

static uint32_t fnv(const char* s) {
  uint32_t h = 2166136261u;
  for (; *s; s++) h = (h ^ (unsigned char)*s) * 16777619u;
  return h;
}

static void indexar(int id) {
  uint32_t i = fnv(formas[id]) & (TABLA - 1);
  while (tabla[i]) i = (i + 1) & (TABLA - 1);
  tabla[i] = id + 1;
}

int idioma_de_forma(const char* f) {
  char buf[FORMA_MAX * 2];
  snprintf(buf, sizeof buf, "%s", f);
  minusculas(buf);
  uint32_t i = fnv(buf) & (TABLA - 1);
  while (tabla[i]) {
    if (strcmp(formas[tabla[i] - 1], buf) == 0) return tabla[i] - 1;
    i = (i + 1) & (TABLA - 1);
  }
  return -1;
}

static uint64_t sm;
static uint64_t splitmix(void) {
  uint64_t z = (sm += 0x9E3779B97F4A7C15ULL);
  z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9ULL;
  z = (z ^ (z >> 27)) * 0x94D049BB133111EBULL;
  return z ^ (z >> 31);
}

static void barajar(char (*v)[5], int n) {
  for (int i = n; i > 1; i--) {
    int j = (int)(splitmix() % (uint64_t)i);
    char tmp[5];
    memcpy(tmp, v[i - 1], 5);
    memcpy(v[i - 1], v[j], 5);
    memcpy(v[j], tmp, 5);
  }
}

void idioma_iniciar(void) {
  static char pares[2500][5], sufijos[30][5];
  const char *C = "ktsnmlrvzp", *V = "aeiou", *S = "dgbfjh";
  int np = 0, ns = 0;
  for (int a = 0; a < 10; a++)
    for (int b = 0; b < 5; b++)
      for (int c = 0; c < 10; c++)
        for (int d = 0; d < 5; d++) {
          char* p = pares[np++];
          p[0] = C[a], p[1] = V[b], p[2] = C[c], p[3] = V[d], p[4] = 0;
        }
  for (int a = 0; a < 6; a++)
    for (int b = 0; b < 5; b++) {
      char* p = sufijos[ns++];
      p[0] = S[a], p[1] = V[b], p[2] = 0;
    }
  sm = 0xAB1A; /* misma semilla siempre: todos nacen con el mismo idioma */
  barajar(pares, np);
  barajar(sufijos, ns);
  memset(tabla, 0, sizeof tabla);
  npalabras = 0;
  for (int r = 0; r < RAICES; r++)
    for (int a = 0; a < ASPECTOS; a++) {
      memcpy(formas[npalabras], pares[r], 5); /* 4 letras + fin */
      if (a) memcpy(formas[npalabras] + 4, sufijos[a - 1], 3);
      indexar(npalabras++);
    }
}

int idioma_tamano(void) { return npalabras; }
int idioma_compuestas(void) { return npalabras - BASE; }
int idioma_valida(int id) { return id >= 0 && id < npalabras; }
int idioma_id(int raiz, int aspecto) { return raiz * ASPECTOS + aspecto; }
int idioma_raiz_de(int id) { return id / ASPECTOS; }
int idioma_es_base(int id) { return id >= 0 && id < BASE; }
const char* idioma_forma(int id) { return idioma_valida(id) ? formas[id] : "?"; }

void idioma_partes(int id, int* a, int* b) {
  *a = comp_a[id - BASE];
  *b = comp_b[id - BASE];
}

void idioma_glosa(int id, Texto* out) {
  if (!idioma_valida(id)) {
    tx_add(out, "?");
  } else if (id >= BASE) {
    idioma_glosa(comp_a[id - BASE], out);
    tx_add(out, "+");
    idioma_glosa(comp_b[id - BASE], out);
  } else {
    tx_add(out, RAIZ[id / ASPECTOS]);
    if (id % ASPECTOS) tx_printf(out, "·%s", ASPECTO[id % ASPECTOS]);
  }
}

const char* idioma_glosa_tmp(int id) {
  static Texto anillo[8];
  static int i = 0;
  Texto* t = &anillo[i++ & 7];
  tx_vaciar(t);
  idioma_glosa(id, t);
  return t->p ? t->p : "";
}

int idioma_de_espanol(const char* w) {
  char buf[64];
  snprintf(buf, sizeof buf, "%s", w);
  minusculas(buf);
  for (int r = 0; r < RAICES; r++)
    if (strcmp(RAIZ[r], buf) == 0) return idioma_id(r, 0);
  return -1;
}

int idioma_relacion(int rel) { return idioma_de_espanol(REL_RAIZ[rel]); }
int idioma_rel_de_palabra(int palabra) {
  for (int r = 0; r < NREL; r++)
    if (idioma_relacion(r) == palabra) return r;
  return -1;
}

int idioma_compuesta_de(int a, int b) {
  for (int i = 0; i < npalabras - BASE; i++)
    if (comp_a[i] == a && comp_b[i] == b) return BASE + i;
  return -1;
}

int idioma_acunar(int a, int b) {
  int ya = idioma_compuesta_de(a, b);
  if (ya >= 0) return ya;
  if (npalabras >= MAX_PALABRAS) return -1;
  char f[FORMA_MAX];
  snprintf(f, sizeof f, "%.2s%.2ssh", formas[a], formas[b]);
  if (idioma_de_forma(f) >= 0) snprintf(f, sizeof f, "%.8s%.8ssh", formas[a], formas[b]);
  while (idioma_de_forma(f) >= 0 && strlen(f) + 1 < FORMA_MAX) strcat(f, "a");
  int id = npalabras++;
  memcpy(formas[id], f, FORMA_MAX);
  comp_a[id - BASE] = a;
  comp_b[id - BASE] = b;
  indexar(id);
  return id;
}

/* Lee texto: acepta palabras de Abla o raíces en español. */
int idioma_leer(const char* texto, int* ids, int max, Texto* desconocidas) {
  int n = 0;
  const char* p = texto;
  while (*p) {
    while (*p && isspace((unsigned char)*p)) p++;
    const char* ini = p;
    while (*p && !isspace((unsigned char)*p)) p++;
    if (p == ini) break;
    char w[64];
    size_t len = (size_t)(p - ini) < sizeof w - 1 ? (size_t)(p - ini) : sizeof w - 1;
    memcpy(w, ini, len);
    w[len] = 0;
    int id = idioma_de_forma(w);
    if (id < 0) id = idioma_de_espanol(w);
    if (id >= 0 && n < max) ids[n++] = id;
    else if (id < 0 && desconocidas) tx_printf(desconocidas, "%s%s", desconocidas->n ? " " : "", w);
  }
  return n;
}

void idioma_frase(const int* ids, int n, Texto* out) {
  for (int i = 0; i < n; i++) tx_printf(out, "%s%s", i ? " " : "", idioma_forma(ids[i]));
}

void idioma_glosa_frase(const int* ids, int n, Texto* out) {
  for (int i = 0; i < n; i++) {
    if (i) tx_add(out, " ");
    idioma_glosa(ids[i], out);
  }
}

void idioma_exportar(const char* ruta) {
  Texto t;
  tx_iniciar(&t);
  tx_printf(&t, "# Diccionario de Abla: %d palabras (%d innatas + %d creadas por la especie)\n# id\tpalabra\tsignificado\n",
            npalabras, BASE, npalabras - BASE);
  for (int i = 0; i < npalabras; i++) {
    tx_printf(&t, "%d\t%s\t", i, formas[i]);
    idioma_glosa(i, &t);
    tx_add(&t, "\n");
  }
  arch_escribir(ruta, t.p, t.n, 0);
  tx_liberar(&t);
}

void idioma_guardar_compuestas(const char* ruta) {
  Texto t;
  tx_iniciar(&t);
  for (int i = BASE; i < npalabras; i++) tx_printf(&t, "%d %s %d %d\n", i, formas[i], comp_a[i - BASE], comp_b[i - BASE]);
  arch_escribir(ruta, t.p ? t.p : "", t.n, 0);
  tx_liberar(&t);
}

void idioma_cargar_compuestas(const char* ruta) {
  Texto t;
  tx_iniciar(&t);
  if (!arch_leer(ruta, &t) || !t.p) {
    tx_liberar(&t);
    return;
  }
  const char* p = t.p;
  int id, a, b, usado;
  char f[FORMA_MAX];
  while (sscanf(p, "%d %23s %d %d%n", &id, f, &a, &b, &usado) == 4) {
    if (id != npalabras || !idioma_valida(a) || !idioma_valida(b) || npalabras >= MAX_PALABRAS) break;
    memcpy(formas[id], f, FORMA_MAX);
    comp_a[id - BASE] = a;
    comp_b[id - BASE] = b;
    npalabras++;
    indexar(id);
    p += usado;
  }
  tx_liberar(&t);
}
