/* Especie Abla — 27 mentes que no dejan de pensar. Versión de terminal.
 *
 * Compilar:  make   (o: cc -std=c11 -O2 con todos los .c de src/ y -lpthread)
 * Usar:      ./abla [--mundo DIR] [--contexto TOKENS] [--ritmo MS] [--consola]
 *
 * En una terminal interactiva abre la interfaz a pantalla completa (tui.c);
 * con --consola, o si la entrada no es una terminal, usa la consola de líneas.
 *
 * Con hilos (Linux, macOS, Termux, iSH): piensan en segundo plano, sin pausa.
 * Sin hilos (WebAssembly, -DABLA_SIN_HILOS): piensan entre cada comando y se
 * ponen al día con todo el tiempo que pasó mientras escribías. */
#include "especie.h"

#if !defined(__wasm__) && !defined(ABLA_SIN_HILOS)
#define ABLA_HILOS 1
#include <pthread.h>
#include <time.h>
static pthread_mutex_t candado = PTHREAD_MUTEX_INITIALIZER;
static volatile int vivo = 1;

static void dormir_ms(int ms) {
  struct timespec ts = {ms / 1000, (long)(ms % 1000) * 1000000L};
  nanosleep(&ts, NULL);
}

static void* pensar_siempre(void* arg) {
  (void)arg;
  while (vivo) {
    pthread_mutex_lock(&candado);
    abla_ciclo();
    pthread_mutex_unlock(&candado);
    dormir_ms(abla_ritmo());
  }
  return NULL;
}
#define TOMAR() pthread_mutex_lock(&candado)
#define SOLTAR() pthread_mutex_unlock(&candado)
#else
#define ABLA_HILOS 0
#define TOMAR() ((void)0)
#define SOLTAR() ((void)0)
#endif


static int color = 1;
static void en_color(const char* codigo, const char* s) {
  if (color) printf("\033[%sm%s\033[0m", codigo, s);
  else fputs(s, stdout);
}

#if ABLA_HILOS
/* Con hilos, «escuchar» muestra en vivo lo que piensan, en tiempo real. */
static void escuchar_en_vivo(int segundos, const char* filtro) {
  TOMAR();
  uint64_t visto = E.nlinea > 10 ? E.nlinea - 10 : 0;
  SOLTAR();
  printf("(escuchando %d s…)\n", segundos);
  for (int paso = 0; paso < segundos * 4; paso++) {
    TOMAR();
    uint64_t desde = E.nlinea > CORRIENTE ? E.nlinea - CORRIENTE : 0;
    if (visto > desde) desde = visto;
    for (uint64_t n = desde + 1; n <= E.nlinea; n++) {
      Linea* l = &E.corriente[(n - 1) % CORRIENTE];
      if (!filtro || strstr(l->texto, filtro)) printf("%s\n", l->texto);
    }
    visto = E.nlinea;
    SOLTAR();
    fflush(stdout);
    dormir_ms(250);
  }
}
#endif

int main(int argc, char** argv) {
  const char* mundo = "mundo";
  unsigned contexto = 1000000;
  int ritmo = 800, consola = 0;
  for (int i = 1; i < argc; i++) {
    if (!strcmp(argv[i], "--mundo") && i + 1 < argc) mundo = argv[++i];
    else if (!strcmp(argv[i], "--contexto") && i + 1 < argc) contexto = (unsigned)strtoul(argv[++i], NULL, 10);
    else if (!strcmp(argv[i], "--ritmo") && i + 1 < argc) ritmo = atoi(argv[++i]);
    else if (!strcmp(argv[i], "--consola")) consola = 1;
    else {
      printf("uso: %s [--mundo DIR] [--contexto TOKENS] [--ritmo MS] [--consola]\n", argv[0]);
      return strcmp(argv[i], "-h") && strcmp(argv[i], "--help");
    }
  }
  setvbuf(stdout, NULL, _IONBF, 0); /* sin búfer: si algo falla, se ve hasta dónde llegó */
  color = getenv("NO_COLOR") == NULL;

  en_color("1;32", "Despertando a la especie…");
  printf(" (%s, carpeta %s)\n", ABLA_HILOS ? "con hilos" : "sin hilos", mundo);
  abla_iniciar(mundo, contexto, 0);
  abla_fijar_ritmo(ritmo);
  printf("Los 27 están despiertos.\n");

  if (!consola && tui_ejecutar(color)) { /* interfaz a pantalla completa */
    abla_guardar();
    printf("La especie dormirá hasta que vuelvas a abrirla, y recordará todo.\n");
    return 0;
  }

  int saludo[3] = {idioma_de_espanol("nosotros"), idioma_de_espanol("despertar"), idioma_de_espanol("pensamiento")};
  Texto a, g;
  tx_iniciar(&a), tx_iniciar(&g);
  idioma_frase(saludo, 3, &a);
  idioma_glosa_frase(saludo, 3, &g);
  printf("\n  Especie Abla · 27 seres · %d palabras · contexto %u tokens por ser\n  ", idioma_tamano(), contexto);
  en_color("35", a.p);
  printf("  («%s»)\n\n  Escribe ayuda para ver los comandos. Ellos ya están pensando.\n\n", g.p);
  tx_liberar(&a), tx_liberar(&g);

#if ABLA_HILOS
  pthread_t hilo;
  pthread_create(&hilo, NULL, pensar_siempre, NULL);
#endif

  char linea[4096];
  for (;;) {
    TOMAR();
    const char* p = abla_prompt();
    char prompt[RUTA_MAX + 32];
    snprintf(prompt, sizeof prompt, "%s", p);
    SOLTAR();
    char* dolar = strrchr(prompt, ':');
    if (dolar) {
      *dolar = 0;
      en_color("1;32", prompt);
      printf(":");
      en_color("1;34", dolar + 1);
    } else {
      fputs(prompt, stdout);
    }
    if (!fgets(linea, sizeof linea, stdin)) break;
    linea[strcspn(linea, "\r\n")] = 0;
    char* c = linea;
    while (*c == ' ') c++;
    if (!strcmp(c, "exit") || !strcmp(c, "salir")) break;
    if (!strcmp(c, "clear")) {
      printf("\033[2J\033[H");
      continue;
    }
#if ABLA_HILOS
    if (!strncmp(c, "escuchar", 8) && (c[8] == 0 || c[8] == ' ')) {
      int seg = 10;
      char filtro[64] = "";
      char** x;
      int n = trocear(c, &x);
      for (int i = 1; i < n; i++) {
        if (isdigit((unsigned char)x[i][0])) seg = atoi(x[i]);
        else {
          TOMAR();
          Ser* s = buscar_ser(x[i]);
          snprintf(filtro, sizeof filtro, "%s", s ? s->nombre : x[i]);
          SOLTAR();
        }
      }
      liberar_args(n, x);
      escuchar_en_vivo(seg < 1 ? 1 : seg, filtro[0] ? filtro : NULL);
      continue;
    }
#endif
    TOMAR();
#if !ABLA_HILOS
    abla_ciclos_atrasados(200);
#endif
    fputs(abla_ejecutar(c), stdout);
    SOLTAR();
  }

  printf("\nGuardando… La especie dormirá hasta que vuelvas a abrirla, y recordará todo.\n");
#if ABLA_HILOS
  vivo = 0;
  pthread_join(hilo, NULL);
#endif
  abla_guardar();
  return 0;
}
