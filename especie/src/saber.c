/* Conocimiento de un ser: hechos "sujeto -relación-> objeto".
 *
 * Se guarda como código C de verdad: conocimiento/<Nombre>.c es un arreglo
 * `const struct hecho Nombre[] = {...}` que compila con cualquier compilador
 * de C, y al despertar el ser vuelve a leer su conocimiento de ese archivo. */
#include "comun.h"

const char* HECHO_H =
    "#ifndef ABLA_HECHO_H\n"
    "#define ABLA_HECHO_H\n"
    "#include <stdint.h>\n\n"
    "/* Un hecho: sujeto -relacion-> objeto.\n"
    " * Sujeto y objeto son ids de palabras de Abla (ver ../abla/diccionario.txt).\n"
    " * Relaciones: 0 es, 1 parte de, 2 causa, 3 igual a, 4 opuesto a.\n"
    " * fuente: 0-26 = otro ser, 100 = innato, 200 = humano. */\n"
    "struct hecho {\n"
    "  uint16_t s;\n"
    "  uint8_t r;\n"
    "  uint16_t o;\n"
    "  float confianza;\n"
    "  uint32_t fuente;\n"
    "};\n\n"
    "#endif\n";

static uint64_t clave(int s, int r, int o) { return (((uint64_t)s << 24) | ((uint64_t)r << 20) | (uint64_t)o) + 1; }

static uint64_t mezclar(uint64_t k) {
  k ^= k >> 33;
  k *= 0xff51afd7ed558ccdULL;
  k ^= k >> 33;
  return k;
}

void saber_iniciar(Saber* s) {
  memset(s, 0, sizeof *s);
  s->tcap = 1024;
  s->claves = xcalloc(s->tcap, sizeof(uint64_t));
  s->valores = xcalloc(s->tcap, sizeof(int32_t));
  s->primero = xmalloc(sizeof(int32_t) * MAX_PALABRAS);
  for (int i = 0; i < MAX_PALABRAS; i++) s->primero[i] = -1;
}

static void tabla_poner(Saber* s, uint64_t k, int32_t v) {
  size_t i = mezclar(k) & (s->tcap - 1);
  while (s->claves[i]) i = (i + 1) & (s->tcap - 1);
  s->claves[i] = k;
  s->valores[i] = v;
}

static int32_t tabla_buscar(const Saber* s, uint64_t k) {
  size_t i = mezclar(k) & (s->tcap - 1);
  while (s->claves[i]) {
    if (s->claves[i] == k) return s->valores[i];
    i = (i + 1) & (s->tcap - 1);
  }
  return -1;
}

static void crecer_tabla(Saber* s) {
  uint64_t* viejas = s->claves;
  int32_t* vals = s->valores;
  size_t vcap = s->tcap;
  s->tcap *= 2;
  s->claves = xcalloc(s->tcap, sizeof(uint64_t));
  s->valores = xcalloc(s->tcap, sizeof(int32_t));
  for (size_t i = 0; i < vcap; i++)
    if (viejas[i]) tabla_poner(s, viejas[i], vals[i]);
  free(viejas);
  free(vals);
}

struct hecho* saber_buscar(Saber* s, int su, int r, int o) {
  int32_t i = tabla_buscar(s, clave(su, r, o));
  return i < 0 ? NULL : &s->h[i];
}

/* Devuelve 1 si el hecho es nuevo. Si ya lo sabía, refuerza la confianza. */
int saber_agregar(Saber* s, struct hecho h) {
  if (h.s == h.o || h.r >= NREL || h.s >= MAX_PALABRAS || h.o >= MAX_PALABRAS) return 0;
  struct hecho* ya = saber_buscar(s, h.s, h.r, h.o);
  if (ya) {
    if (h.confianza > ya->confianza) ya->confianza = h.confianza;
    return 0;
  }
  if (s->n >= MAX_HECHOS) return 0;
  if (s->n == s->cap) {
    s->cap = s->cap ? s->cap * 2 : 64;
    s->h = xrealloc(s->h, sizeof(struct hecho) * s->cap);
    s->siguiente = xrealloc(s->siguiente, sizeof(int32_t) * s->cap);
  }
  if ((s->n + 1) * 2 > s->tcap) crecer_tabla(s);
  int32_t i = (int32_t)s->n++;
  s->h[i] = h;
  s->siguiente[i] = s->primero[h.s];
  s->primero[h.s] = i;
  tabla_poner(s, clave(h.s, h.r, h.o), i);
  return 1;
}

/* Índices de los hechos con ese sujeto y relación (r < 0: cualquier relación). */
int saber_desde(const Saber* s, int su, int r, int* out, int max) {
  int n = 0;
  if (su < 0 || su >= MAX_PALABRAS) return 0;
  for (int32_t i = s->primero[su]; i >= 0 && n < max; i = s->siguiente[i])
    if (r < 0 || s->h[i].r == r) out[n++] = i;
  return n;
}

int saber_exportar_c(const Saber* s, const char* ruta, const char* nombre, const char* encabezado) {
  Texto t;
  tx_iniciar(&t);
  tx_printf(&t,
            "/* %s\n"
            " * Este archivo es C válido y es, a la vez, la memoria de este ser:\n"
            " * al despertar vuelve a leer de aquí todo lo que sabe. */\n"
            "#include \"hecho.h\"\n\n"
            "const struct hecho %s[] = {\n",
            encabezado, nombre);
  if (!s->n) tx_add(&t, "    {0, 0, 0, 0.000f, 0}, /* (vacío) */\n");
  for (size_t i = 0; i < s->n; i++) {
    const struct hecho* h = &s->h[i];
    tx_printf(&t, "    {%5u, %u, %5u, %.3ff, %3u}, /* %s %s %s */\n", h->s, h->r, h->o, (double)h->confianza,
              (unsigned)h->fuente, idioma_glosa_tmp(h->s), rel_glosa(h->r), idioma_glosa_tmp(h->o));
  }
  tx_printf(&t, "};\n\nconst unsigned %s_n = sizeof %s / sizeof %s[0];\n", nombre, nombre, nombre);
  char tmp[RUTA_MAX + 8];
  snprintf(tmp, sizeof tmp, "%s.tmp", ruta);
  int ok = arch_escribir(tmp, t.p, t.n, 0) && arch_renombrar(tmp, ruta);
  tx_liberar(&t);
  return ok;
}

int saber_importar_c(Saber* s, const char* ruta) {
  Texto t;
  tx_iniciar(&t);
  if (!arch_leer(ruta, &t)) {
    tx_liberar(&t);
    return 0;
  }
  for (char* p = t.p; p && *p;) {
    char* fin = strchr(p, '\n');
    if (fin) *fin = 0;
    char* llave = strchr(p, '{');
    unsigned su, r, o, f;
    float c;
    if (llave && sscanf(llave, " { %u , %u , %u , %ff , %u }", &su, &r, &o, &c, &f) == 5 && c > 0 &&
        idioma_valida((int)su) && idioma_valida((int)o)) {
      struct hecho h = {(uint16_t)su, (uint8_t)r, (uint16_t)o, c, f};
      saber_agregar(s, h);
    }
    p = fin ? fin + 1 : NULL;
  }
  tx_liberar(&t);
  return 1;
}
