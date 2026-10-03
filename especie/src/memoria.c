/* Memoria de un ser.
 *  - Contexto: los últimos N tokens de recuerdos, en RAM (1.000.000 por defecto).
 *  - Largo plazo: todo lo que piensa, en un archivo que solo crece. Nunca
 *    olvida; solo deja de tenerlo presente. (En la versión web, donde el
 *    espacio del navegador es limitado, se guardan los dos últimos tramos.)
 * Al despertar, el contexto se reconstruye leyendo el archivo. */
#include "comun.h"

static uint32_t contar_tokens(const char* s) {
  uint32_t n = 0;
  int dentro = 0;
  for (; *s; s++) {
    int esp = *s == ' ' || *s == '\t' || *s == '\n';
    if (!esp && !dentro) n++;
    dentro = !esp;
  }
  return n ? n : 1;
}

static void meter(Memoria* m, uint64_t tick, const char* texto) {
  if (m->n == m->cap) {
    size_t cap = m->cap ? m->cap * 2 : 256;
    Recuerdo* r = xmalloc(sizeof(Recuerdo) * cap);
    for (size_t i = 0; i < m->n; i++) r[i] = m->r[(m->ini + i) % m->cap];
    free(m->r);
    m->r = r;
    m->cap = cap;
    m->ini = 0;
  }
  Recuerdo* x = &m->r[(m->ini + m->n) % m->cap];
  x->tick = tick;
  x->texto = xstrdup(texto);
  x->tokens = contar_tokens(texto);
  m->n++;
  m->usados += x->tokens;
  while (m->usados > m->capacidad && m->n > 1) {
    Recuerdo* v = &m->r[m->ini];
    m->usados -= v->tokens;
    free(v->texto);
    m->ini = (m->ini + 1) % m->cap;
    m->n--;
  }
}

static void cargar(Memoria* m, const char* ruta) {
  FILE* f = fopen(ruta, "rb");
  if (!f) return;
  Texto linea;
  tx_iniciar(&linea);
  int c;
  for (;;) {
    tx_vaciar(&linea);
    while ((c = fgetc(f)) != EOF && c != '\n') {
      char ch = (char)c;
      tx_addn(&linea, &ch, 1);
    }
    if (linea.n) {
      char* tab = strchr(linea.p, '\t');
      if (tab) {
        m->total++;
        meter(m, strtoull(linea.p, NULL, 10), tab + 1);
      }
    }
    if (c == EOF) break;
  }
  tx_liberar(&linea);
  fclose(f);
}

void memoria_iniciar(Memoria* m, uint64_t capacidad, const char* ruta, long max_bytes) {
  memset(m, 0, sizeof *m);
  m->capacidad = capacidad;
  m->max_bytes = max_bytes;
  snprintf(m->ruta, sizeof m->ruta, "%s", ruta);
  char viejo[RUTA_MAX + 4];
  snprintf(viejo, sizeof viejo, "%s.1", ruta);
  cargar(m, viejo);
  cargar(m, ruta);
  m->log = fopen(ruta, "ab");
}

void memoria_recordar(Memoria* m, uint64_t tick, const char* texto) {
  char* limpio = xstrdup(texto);
  for (char* p = limpio; *p; p++)
    if (*p == '\n' || *p == '\t') *p = ' ';
  if (m->log) {
    fprintf(m->log, "%llu\t%s\n", (unsigned long long)tick, limpio);
    if (m->max_bytes > 0 && ftell(m->log) > m->max_bytes) {
      char viejo[RUTA_MAX + 4];
      snprintf(viejo, sizeof viejo, "%s.1", m->ruta);
      fclose(m->log);
      remove(viejo);
      rename(m->ruta, viejo);
      m->log = fopen(m->ruta, "ab");
    }
  }
  m->total++;
  meter(m, tick, limpio);
  free(limpio);
}

const Recuerdo* memoria_en(const Memoria* m, size_t i) { return &m->r[(m->ini + i) % m->cap]; }

long long memoria_bytes(const Memoria* m) {
  char viejo[RUTA_MAX + 4];
  snprintf(viejo, sizeof viejo, "%s.1", m->ruta);
  long long a = arch_tamano(m->ruta), b = arch_tamano(viejo);
  return (a > 0 ? a : 0) + (b > 0 ? b : 0);
}

void memoria_sincronizar(Memoria* m) {
  if (m->log) fflush(m->log);
}

static int contiene(const char* texto, const char* q) {
  /* búsqueda sin distinguir mayúsculas (ASCII) */
  size_t lq = strlen(q);
  for (const char* p = texto; *p; p++) {
    size_t i = 0;
    while (i < lq && p[i] && tolower((unsigned char)p[i]) == tolower((unsigned char)q[i])) i++;
    if (i == lq) return 1;
  }
  return 0;
}

static void buscar_en_archivo(const char* ruta, const char* q, size_t max, size_t* hallados, uint64_t limite,
                              uint64_t* leidas, Texto* out) {
  FILE* f = fopen(ruta, "rb");
  if (!f) return;
  Texto l;
  tx_iniciar(&l);
  int c;
  while (*hallados < max && *leidas < limite) {
    tx_vaciar(&l);
    while ((c = fgetc(f)) != EOF && c != '\n') {
      char ch = (char)c;
      tx_addn(&l, &ch, 1);
    }
    if (!l.n && c == EOF) break;
    (*leidas)++;
    char* tab = l.p ? strchr(l.p, '\t') : NULL;
    if (tab && contiene(tab + 1, q)) {
      *tab = 0;
      tx_printf(out, "[t=%s, largo plazo] %s\n", l.p, tab + 1);
      (*hallados)++;
    }
    if (c == EOF) break;
  }
  tx_liberar(&l);
  fclose(f);
}

void memoria_buscar(Memoria* m, const char* q, size_t max, Texto* out) {
  size_t hallados = 0;
  for (size_t i = m->n; i-- > 0 && hallados < max;) {
    const Recuerdo* r = memoria_en(m, i);
    if (contiene(r->texto, q)) {
      tx_printf(out, "[t=%llu] %s\n", (unsigned long long)r->tick, r->texto);
      hallados++;
    }
  }
  if (hallados < max && m->total > m->n) {
    memoria_sincronizar(m);
    char viejo[RUTA_MAX + 4];
    snprintf(viejo, sizeof viejo, "%s.1", m->ruta);
    uint64_t leidas = 0, limite = m->total - m->n;
    buscar_en_archivo(viejo, q, max, &hallados, limite, &leidas, out);
    buscar_en_archivo(m->ruta, q, max, &hallados, limite, &leidas, out);
  }
}
