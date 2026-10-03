/* Carpetas y archivos con POSIX (stat, mkdir, opendir) y stdio.
 * Sin <sys/types.h> ni <unistd.h>: en algunos entornos WebAssembly faltan. */
#include "comun.h"

#include <dirent.h>
#include <sys/stat.h>

void ruta_unir(char* out, size_t n, const char* a, const char* b) {
  if (!*a) snprintf(out, n, "%s", b);
  else if (!*b) snprintf(out, n, "%s", a);
  else {
    size_t la = strlen(a);
    int barra_a = a[la - 1] == '/', barra_b = b[0] == '/';
    if (barra_a && barra_b) snprintf(out, n, "%s%s", a, b + 1);
    else if (barra_a || barra_b) snprintf(out, n, "%s%s", a, b);
    else snprintf(out, n, "%s/%s", a, b);
  }
}

const char* ruta_nombre(const char* p) {
  const char* b = strrchr(p, '/');
  return b ? b + 1 : p;
}

int arch_existe(const char* p) {
  struct stat st;
  return stat(p, &st) == 0;
}
int arch_es_directorio(const char* p) {
  struct stat st;
  return stat(p, &st) == 0 && S_ISDIR(st.st_mode);
}
int arch_es_archivo(const char* p) {
  struct stat st;
  return stat(p, &st) == 0 && S_ISREG(st.st_mode);
}
long long arch_tamano(const char* p) {
  struct stat st;
  return stat(p, &st) == 0 ? (long long)st.st_size : -1;
}

int arch_crear_directorios(const char* p) {
  if (!*p || arch_es_directorio(p)) return 1;
  char parcial[RUTA_MAX];
  size_t n = strlen(p);
  for (size_t i = 1; i <= n && i < RUTA_MAX; i++) {
    if (i == n || p[i] == '/') {
      memcpy(parcial, p, i);
      parcial[i] = 0;
      if (!arch_es_directorio(parcial) && mkdir(parcial, 0755) != 0 && !arch_es_directorio(parcial)) return 0;
    }
  }
  return 1;
}

static int por_nombre(const void* a, const void* b) {
  return strcmp(((const Entrada*)a)->nombre, ((const Entrada*)b)->nombre);
}

int arch_listar(const char* p, Entrada** out) {
  *out = NULL;
  DIR* d = opendir(p);
  if (!d) return 0;
  int n = 0, cap = 0;
  struct dirent* e;
  while ((e = readdir(d))) {
    if (!strcmp(e->d_name, ".") || !strcmp(e->d_name, "..")) continue;
    if (n == cap) {
      cap = cap ? cap * 2 : 16;
      *out = xrealloc(*out, sizeof(Entrada) * (size_t)cap);
    }
    Entrada* x = &(*out)[n++];
    snprintf(x->nombre, sizeof x->nombre, "%s", e->d_name);
    char ruta[RUTA_MAX];
    ruta_unir(ruta, sizeof ruta, p, e->d_name);
    x->directorio = arch_es_directorio(ruta);
    x->tamano = arch_tamano(ruta);
  }
  closedir(d);
  if (n) qsort(*out, (size_t)n, sizeof(Entrada), por_nombre);
  return n;
}

int arch_borrar_todo(const char* p) {
  if (arch_es_directorio(p)) {
    Entrada* e;
    int n = arch_listar(p, &e);
    for (int i = 0; i < n; i++) {
      char r[RUTA_MAX];
      ruta_unir(r, sizeof r, p, e[i].nombre);
      arch_borrar_todo(r);
    }
    free(e);
  }
  return remove(p) == 0; /* remove() también borra carpetas vacías */
}

int arch_renombrar(const char* a, const char* b) { return rename(a, b) == 0; }

int arch_leer(const char* p, Texto* out) {
  tx_vaciar(out);
  FILE* f = fopen(p, "rb");
  if (!f) return 0;
  char buf[8192];
  size_t n;
  while ((n = fread(buf, 1, sizeof buf, f)) > 0) tx_addn(out, buf, n);
  fclose(f);
  if (!out->p) tx_add(out, "");
  return 1;
}

int arch_escribir(const char* p, const char* datos, size_t n, int anexar) {
  FILE* f = fopen(p, anexar ? "ab" : "wb");
  if (!f) return 0;
  int ok = fwrite(datos, 1, n, f) == n;
  return fclose(f) == 0 && ok;
}

int arch_copiar(const char* a, const char* b) {
  if (arch_es_directorio(a)) {
    if (!arch_crear_directorios(b)) return 0;
    Entrada* e;
    int n = arch_listar(a, &e), ok = 1;
    for (int i = 0; i < n; i++) {
      char x[RUTA_MAX], y[RUTA_MAX];
      ruta_unir(x, sizeof x, a, e[i].nombre);
      ruta_unir(y, sizeof y, b, e[i].nombre);
      ok = arch_copiar(x, y) && ok;
    }
    free(e);
    return ok;
  }
  Texto t;
  tx_iniciar(&t);
  int ok = arch_leer(a, &t) && arch_escribir(b, t.p, t.n, 0);
  tx_liberar(&t);
  return ok;
}
