/* Terminal estilo bash/Termux, encerrada en la carpeta del mundo.
 * La usan el humano y los 27 seres (cada uno con su $HOME).
 * Trabaja con rutas "virtuales" que empiezan en "/" (la raíz del mundo);
 * nunca se puede salir de ahí. */
#include "comun.h"

/* Divide una línea respetando "comillas" y 'comillas'. */
int trocear(const char* linea, char*** argv) {
  int n = 0, cap = 8;
  char** v = xmalloc(sizeof(char*) * (size_t)cap);
  Texto actual;
  tx_iniciar(&actual);
  char comilla = 0;
  int hay = 0;
  for (const char* p = linea;; p++) {
    char c = *p;
    int fin = c == 0;
    if (!fin && comilla) {
      if (c == comilla) comilla = 0;
      else tx_addn(&actual, &c, 1);
    } else if (!fin && (c == '"' || c == '\'')) {
      comilla = c;
      hay = 1;
    } else if (fin || c == ' ' || c == '\t' || c == '\n' || c == '\r') {
      if (hay || actual.n) {
        if (n + 1 >= cap) v = xrealloc(v, sizeof(char*) * (size_t)(cap *= 2));
        v[n++] = xstrdup(actual.p ? actual.p : "");
      }
      tx_vaciar(&actual);
      hay = 0;
    } else {
      tx_addn(&actual, &c, 1);
    }
    if (fin) break;
  }
  tx_liberar(&actual);
  v[n] = NULL;
  *argv = v;
  return n;
}

void liberar_args(int argc, char** argv) {
  for (int i = 0; i < argc; i++) free(argv[i]);
  free(argv);
}

void shell_iniciar(Shell* sh, const char* raiz, const char* home) {
  snprintf(sh->raiz, sizeof sh->raiz, "%s", raiz);
  snprintf(sh->home, sizeof sh->home, "%s", home);
  snprintf(sh->cwd, sizeof sh->cwd, "%s", home);
  char r[RUTA_MAX];
  shell_real(sh, home, r, sizeof r);
  arch_crear_directorios(r);
}

void shell_real(const Shell* sh, const char* virt, char* out, size_t n) {
  if (!strcmp(virt, "/")) snprintf(out, n, "%s", sh->raiz);
  else ruta_unir(out, n, sh->raiz, virt);
}

const char* shell_mostrar(const Shell* sh, const char* v) {
  return (!strcmp(v, sh->home) && strcmp(sh->home, "/")) ? "~" : v;
}

/* Ruta escrita por el usuario → ruta virtual normalizada. 0 si intenta salir del mundo. */
int shell_resolver(const Shell* sh, const char* arg, char* out, size_t n) {
  char completo[RUTA_MAX * 2];
  if (!*arg || !strcmp(arg, "~")) snprintf(completo, sizeof completo, "%s", sh->home);
  else if (!strncmp(arg, "~/", 2)) snprintf(completo, sizeof completo, "%s/%s", sh->home, arg + 2);
  else if (arg[0] == '/') snprintf(completo, sizeof completo, "%s", arg);
  else snprintf(completo, sizeof completo, "%s/%s", sh->cwd, arg);

  char* partes[128];
  int np = 0;
  char* guardar = NULL;
  for (char* p = strtok_r(completo, "/", &guardar); p; p = strtok_r(NULL, "/", &guardar)) {
    if (!strcmp(p, ".")) continue;
    if (!strcmp(p, "..")) {
      if (!np) return 0;
      np--;
    } else if (np < 128) {
      partes[np++] = p;
    }
  }
  size_t len = 0;
  out[0] = 0;
  for (int i = 0; i < np; i++) {
    int w = snprintf(out + len, n - len, "/%s", partes[i]);
    if (w < 0 || (size_t)w >= n - len) return 0;
    len += (size_t)w;
  }
  if (!np) snprintf(out, n, "/");
  return 1;
}

/* Resuelve y deja la ruta real; si falla, escribe el error. */
static int ruta(const Shell* sh, const char* cmd, const char* arg, char* real, char* virt, Texto* out) {
  char v[RUTA_MAX];
  if (!shell_resolver(sh, arg, v, sizeof v)) {
    tx_printf(out, "%s: %s: fuera del mundo\n", cmd, arg);
    return 0;
  }
  if (virt) snprintf(virt, RUTA_MAX, "%s", v);
  shell_real(sh, v, real, RUTA_MAX);
  return 1;
}

static void cmd_ls(Shell* sh, int argc, char** argv, Texto* out) {
  int largo = 0;
  const char* arg = ".";
  for (int i = 1; i < argc; i++) {
    if (!strcmp(argv[i], "-l") || !strcmp(argv[i], "-la") || !strcmp(argv[i], "-al")) largo = 1;
    else if (argv[i][0] != '-') arg = argv[i];
  }
  char r[RUTA_MAX];
  if (!ruta(sh, "ls", arg, r, NULL, out)) return;
  if (!arch_existe(r)) {
    tx_printf(out, "ls: %s: no existe\n", arg);
    return;
  }
  if (!arch_es_directorio(r)) {
    tx_printf(out, "%s\n", ruta_nombre(r));
    return;
  }
  Entrada* e;
  int n = arch_listar(r, &e);
  for (int i = 0; i < n; i++) {
    if (largo) {
      if (e[i].directorio) tx_printf(out, "d %10s  %s/\n", "-", e[i].nombre);
      else tx_printf(out, "- %10lld  %s\n", e[i].tamano, e[i].nombre);
    } else {
      tx_printf(out, "%s%s  ", e[i].nombre, e[i].directorio ? "/" : "");
    }
  }
  if (!largo && n) tx_add(out, "\n");
  free(e);
}

static void arbol(const char* p, const char* pre, int nivel, Texto* out, int* cuenta) {
  if (nivel > 3) return;
  Entrada* e;
  int n = arch_listar(p, &e);
  for (int i = 0; i < n; i++) {
    if (++*cuenta > 400) {
      tx_printf(out, "%s…\n", pre);
      break;
    }
    int ultimo = i + 1 == n;
    tx_printf(out, "%s%s%s%s\n", pre, ultimo ? "└── " : "├── ", e[i].nombre, e[i].directorio ? "/" : "");
    if (e[i].directorio) {
      char hijo[RUTA_MAX], pre2[256];
      ruta_unir(hijo, sizeof hijo, p, e[i].nombre);
      snprintf(pre2, sizeof pre2, "%s%s", pre, ultimo ? "    " : "│   ");
      arbol(hijo, pre2, nivel + 1, out, cuenta);
    }
  }
  free(e);
}

static void cmd_cabeza(Shell* sh, int argc, char** argv, Texto* out, int cola) {
  long n = 10;
  const char* arg = "";
  for (int i = 1; i < argc; i++) {
    if (!strcmp(argv[i], "-n") && i + 1 < argc) n = strtol(argv[++i], NULL, 10);
    else arg = argv[i];
  }
  char r[RUTA_MAX];
  if (!ruta(sh, argv[0], arg, r, NULL, out)) return;
  Texto t;
  tx_iniciar(&t);
  if (!arch_es_archivo(r) || !arch_leer(r, &t)) {
    tx_printf(out, "%s: %s: no es un archivo\n", argv[0], arg);
    tx_liberar(&t);
    return;
  }
  /* inicio de cada línea */
  long total = 0, cap = 64;
  size_t* ini = xmalloc(sizeof(size_t) * (size_t)cap);
  for (size_t i = 0; i < t.n; i = strchr(t.p + i, '\n') ? (size_t)(strchr(t.p + i, '\n') - t.p) + 1 : t.n) {
    if (total == cap) ini = xrealloc(ini, sizeof(size_t) * (size_t)(cap *= 2));
    ini[total++] = i;
  }
  long desde = cola ? (total > n ? total - n : 0) : 0, hasta = cola ? total : (n < total ? n : total);
  for (long k = desde; k < hasta; k++) {
    size_t fin = k + 1 < total ? ini[k + 1] : t.n;
    tx_addn(out, t.p + ini[k], fin - ini[k]);
    if (fin == t.n && (!t.n || t.p[t.n - 1] != '\n')) tx_add(out, "\n");
  }
  free(ini);
  tx_liberar(&t);
}

int shell_ejecutar(Shell* sh, int argc, char** argv, Texto* out) {
  if (!argc) return 1;
  const char* c = argv[0];
  char r[RUTA_MAX], v[RUTA_MAX];

  if (!strcmp(c, "pwd")) {
    tx_printf(out, "%s\n", sh->cwd);
  } else if (!strcmp(c, "cd")) {
    const char* arg = argc > 1 ? argv[1] : "~";
    if (ruta(sh, "cd", arg, r, v, out)) {
      if (!arch_es_directorio(r)) tx_printf(out, "cd: %s: no es un directorio\n", arg);
      else snprintf(sh->cwd, sizeof sh->cwd, "%s", v);
    }
  } else if (!strcmp(c, "ls")) {
    cmd_ls(sh, argc, argv, out);
  } else if (!strcmp(c, "cat")) {
    for (int i = 1; i < argc; i++) {
      if (!ruta(sh, "cat", argv[i], r, NULL, out)) continue;
      Texto t;
      tx_iniciar(&t);
      if (arch_es_archivo(r) && arch_leer(r, &t)) tx_addn(out, t.p, t.n);
      else tx_printf(out, "cat: %s: no es un archivo\n", argv[i]);
      tx_liberar(&t);
    }
  } else if (!strcmp(c, "head") || !strcmp(c, "tail")) {
    cmd_cabeza(sh, argc, argv, out, !strcmp(c, "tail"));
  } else if (!strcmp(c, "mkdir")) {
    for (int i = 1; i < argc; i++)
      if (strcmp(argv[i], "-p") && ruta(sh, "mkdir", argv[i], r, NULL, out) && !arch_crear_directorios(r))
        tx_printf(out, "mkdir: %s: no se pudo crear\n", argv[i]);
  } else if (!strcmp(c, "touch")) {
    for (int i = 1; i < argc; i++)
      if (ruta(sh, "touch", argv[i], r, NULL, out)) arch_escribir(r, "", 0, 1);
  } else if (!strcmp(c, "rm")) {
    int recursivo = 0;
    for (int i = 1; i < argc; i++) {
      if (!strcmp(argv[i], "-r") || !strcmp(argv[i], "-rf") || !strcmp(argv[i], "-f")) {
        recursivo = 1;
        continue;
      }
      if (!ruta(sh, "rm", argv[i], r, v, out)) continue;
      if (!strcmp(v, "/") || !strcmp(v, sh->home)) tx_printf(out, "rm: no se puede borrar %s\n", argv[i]);
      else if (!arch_existe(r)) tx_printf(out, "rm: %s: no existe\n", argv[i]);
      else if (arch_es_directorio(r) && !recursivo) tx_printf(out, "rm: %s: es un directorio (usa -r)\n", argv[i]);
      else arch_borrar_todo(r);
    }
  } else if (!strcmp(c, "mv") || !strcmp(c, "cp")) {
    char d[RUTA_MAX];
    if (argc != 3) tx_printf(out, "%s: uso: %s origen destino\n", c, c);
    else if (ruta(sh, c, argv[1], r, NULL, out) && ruta(sh, c, argv[2], d, NULL, out)) {
      if (!arch_existe(r)) tx_printf(out, "%s: %s: no existe\n", c, argv[1]);
      else {
        if (arch_es_directorio(d)) {
          char d2[RUTA_MAX];
          ruta_unir(d2, sizeof d2, d, ruta_nombre(r));
          snprintf(d, sizeof d, "%s", d2);
        }
        int ok = !strcmp(c, "cp") ? arch_copiar(r, d) : arch_renombrar(r, d);
        if (!ok) tx_printf(out, "%s: no se pudo\n", c);
      }
    }
  } else if (!strcmp(c, "echo")) {
    Texto t;
    tx_iniciar(&t);
    const char* destino = NULL;
    int anexar = 0;
    for (int i = 1; i < argc; i++) {
      if ((!strcmp(argv[i], ">") || !strcmp(argv[i], ">>")) && i + 1 < argc) {
        anexar = !strcmp(argv[i], ">>");
        destino = argv[++i];
        continue;
      }
      tx_printf(&t, "%s%s", t.n ? " " : "", argv[i]);
    }
    tx_add(&t, "\n");
    if (!destino) tx_addn(out, t.p, t.n);
    else if (ruta(sh, "echo", destino, r, NULL, out)) arch_escribir(r, t.p, t.n, anexar);
    tx_liberar(&t);
  } else if (!strcmp(c, "tree")) {
    if (ruta(sh, "tree", argc > 1 ? argv[1] : ".", r, v, out)) {
      int cuenta = 0;
      tx_printf(out, "%s\n", v);
      arbol(r, "", 0, out, &cuenta);
    }
  } else if (!strcmp(c, "wc")) {
    for (int i = 1; i < argc; i++) {
      if (!ruta(sh, "wc", argv[i], r, NULL, out)) continue;
      Texto t;
      tx_iniciar(&t);
      arch_leer(r, &t);
      size_t l = 0, w = 0;
      int dentro = 0;
      for (size_t k = 0; k < t.n; k++) {
        if (t.p[k] == '\n') l++;
        int esp = isspace((unsigned char)t.p[k]);
        if (!esp && !dentro) w++;
        dentro = !esp;
      }
      tx_printf(out, "%zu %zu %zu %s\n", l, w, t.n, argv[i]);
      tx_liberar(&t);
    }
  } else if (!strcmp(c, "grep")) {
    if (argc < 3) tx_add(out, "grep: uso: grep patrón archivo...\n");
    for (int i = 2; i < argc; i++) {
      if (!ruta(sh, "grep", argv[i], r, NULL, out)) continue;
      Texto t;
      tx_iniciar(&t);
      arch_leer(r, &t);
      for (char* p = t.p; p && *p;) {
        char* fin = strchr(p, '\n');
        if (fin) *fin = 0;
        if (strstr(p, argv[1])) tx_printf(out, "%s%s%s\n", argc > 3 ? argv[i] : "", argc > 3 ? ":" : "", p);
        p = fin ? fin + 1 : NULL;
      }
      tx_liberar(&t);
    }
  } else {
    return 0;
  }
  return 1;
}
