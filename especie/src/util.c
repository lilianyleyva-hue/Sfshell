/* Utilidades: reservas que no fallan en silencio y texto que crece. */
#include "comun.h"

void* xmalloc(size_t n) {
  void* p = malloc(n ? n : 1);
  if (!p) {
    fprintf(stderr, "Abla: sin memoria\n");
    abort();
  }
  return p;
}

void* xcalloc(size_t n, size_t t) {
  void* p = calloc(n ? n : 1, t ? t : 1);
  if (!p) {
    fprintf(stderr, "Abla: sin memoria\n");
    abort();
  }
  return p;
}

void* xrealloc(void* p, size_t n) {
  void* q = realloc(p, n ? n : 1);
  if (!q) {
    fprintf(stderr, "Abla: sin memoria\n");
    abort();
  }
  return q;
}

char* xstrdup(const char* s) {
  size_t n = strlen(s) + 1;
  char* p = xmalloc(n);
  memcpy(p, s, n);
  return p;
}

void tx_iniciar(Texto* t) {
  t->p = NULL;
  t->n = t->cap = 0;
}

void tx_liberar(Texto* t) {
  free(t->p);
  tx_iniciar(t);
}

void tx_vaciar(Texto* t) {
  t->n = 0;
  if (t->p) t->p[0] = 0;
}

static void tx_reservar(Texto* t, size_t extra) {
  if (t->n + extra + 1 <= t->cap) return;
  size_t cap = t->cap ? t->cap : 64;
  while (cap < t->n + extra + 1) cap *= 2;
  t->p = xrealloc(t->p, cap);
  t->cap = cap;
}

void tx_addn(Texto* t, const char* s, size_t n) {
  tx_reservar(t, n);
  memcpy(t->p + t->n, s, n);
  t->n += n;
  t->p[t->n] = 0;
}

void tx_add(Texto* t, const char* s) { tx_addn(t, s, strlen(s)); }

void tx_printf(Texto* t, const char* fmt, ...) {
  va_list ap;
  va_start(ap, fmt);
  va_list ap2;
  va_copy(ap2, ap);
  int n = vsnprintf(NULL, 0, fmt, ap);
  va_end(ap);
  if (n > 0) {
    tx_reservar(t, (size_t)n);
    vsnprintf(t->p + t->n, (size_t)n + 1, fmt, ap2);
    t->n += (size_t)n;
  }
  va_end(ap2);
}

void tx_json(Texto* t, const char* s) {
  tx_add(t, "\"");
  for (; *s; s++) {
    unsigned char c = (unsigned char)*s;
    if (c == '"') tx_add(t, "\\\"");
    else if (c == '\\') tx_add(t, "\\\\");
    else if (c == '\n') tx_add(t, "\\n");
    else if (c == '\t') tx_add(t, "\\t");
    else if (c < 0x20) tx_printf(t, "\\u%04x", c);
    else tx_addn(t, (const char*)&c, 1);
  }
  tx_add(t, "\"");
}

void minusculas(char* s) {
  for (; *s; s++) *s = (char)tolower((unsigned char)*s);
}
