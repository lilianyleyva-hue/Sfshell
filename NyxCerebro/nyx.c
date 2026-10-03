/* nyx.c — Interfaz de texto de las 18 IAs de Nyx (C99).
 *
 * Se ejecuta en la terminal de Code App (o en cualquier terminal):
 *   abre nyx.c y pulsa ▶   ·   o:  clang nyx.c -o nyx -lm && ./nyx
 * Necesita cerebro.h y resh.h en la misma carpeta.
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include "cerebro.h"

#define RUTA_MEMORIA "nyx_memoria.txt"

/* las 18 mentes viven en memoria pedida al arrancar (no en el programa):
 * así el runtime de WebAssembly de Code App no tiene que reservarla de golpe */
static Consejo *nyxp;
#define nyx (*nyxp)
static int conColor = 1;
static int verAprende = 1;

/* ------------------------------------------------------------------ */
/*  Colores y dibujo                                                   */
/* ------------------------------------------------------------------ */

static void color(int c) { if (conColor) printf("\033[38;5;%dm", c); }
static void negrita(void) { if (conColor) printf("\033[1m"); }
static void tenue(void) { if (conColor) printf("\033[2m"); }
static void normal(void) { if (conColor) printf("\033[0m"); }

static void nombre_rol(int r, int ancho) {
    color(NYX_ROLES[r].color);
    printf("%-*s", ancho, NYX_ROLES[r].nombre);
    normal();
}

static void barra(float x, float max, int ancho, int c) {
    int i, llenas = max > 0 ? (int)(x / max * (float)ancho + 0.5f) : 0;
    if (llenas < 0) llenas = 0;
    if (llenas > ancho) llenas = ancho;
    color(c);
    for (i = 0; i < llenas; i++) printf("█");
    tenue();
    for (; i < ancho; i++) printf("░");
    normal();
}

static void linea_caja(const char *izq, const char *der, int ancho) {
    int i;
    color(99);
    printf("%s", izq);
    for (i = 0; i < ancho; i++) printf("─");
    printf("%s\n", der);
    normal();
}

static int ancho_utf8(const char *s) {
    int n = 0;
    for (; *s; s++) if (((unsigned char)*s & 0xC0) != 0x80) n++;
    return n;
}

static void fila_caja(const char *texto, int ancho, int c) {
    int pad = ancho - 2 - ancho_utf8(texto);
    color(99); printf("│ "); normal();
    color(c); printf("%s", texto); normal();
    while (pad-- > 0) printf(" ");
    color(99); printf(" │\n"); normal();
}

static void banner(void) {
    const int A = 58;
    printf("\n");
    linea_caja("╭", "╮", A);
    negrita();
    fila_caja("N Y X  ·  cerebro resonante de 18 mentes", A, 219);
    fila_caja("cada una piensa, imagina, aprende y habla Resh", A, 117);
    fila_caja("escribe una pregunta, o  ayuda", A, 250);
    linea_caja("╰", "╯", A);
}

/* ------------------------------------------------------------------ */
/*  Lo que pasa dentro (eventos del cerebro)                           */
/* ------------------------------------------------------------------ */

static const char *icono(int tipo) {
    switch (tipo) {
    case EV_PIENSA: return "💭";
    case EV_DICE: return "▶ ";
    case EV_APRENDE: return "📚";
    case EV_IDEA: return "✨";
    case EV_RESH: return "📖";
    case EV_SUENA: return "🌙";
    default: return "· ";
    }
}

static void al_evento(void *ud, int rol, int tipo, const char *texto) {
    (void)ud;
    if (tipo == EV_APRENDE && !verAprende) return;
    tenue(); printf("%5lu ", nyx.tick); normal();
    nombre_rol(rol, 11);
    printf(" %s ", icono(tipo));
    if (tipo == EV_DICE) { negrita(); printf("%s", texto); normal(); }
    else if (tipo == EV_APRENDE || tipo == EV_NOTA) { tenue(); printf("%s", texto); normal(); }
    else printf("%s", texto);
    printf("\n");
    fflush(stdout);
}

/* ------------------------------------------------------------------ */
/*  Pantallas                                                          */
/* ------------------------------------------------------------------ */

static void panel_mentes(void) {
    int r;
    printf("\n");
    tenue(); printf("  %-11s %-17s %6s %6s %5s  %s\n", "mente", "precisión", "ideas", "Resh", "dijo", "lo que más le gusta"); normal();
    for (r = 0; r < NROLES; r++) {
        const Mente *m = &nyx.m[r];
        float p = m_precision(m);
        printf("  ");
        nombre_rol(r, 11);
        printf(" ");
        barra(p, 1, 10, NYX_ROLES[r].color);
        printf(" %3d%% %6d %6d %5ld  %s\n", (int)(p * 100 + 0.5f), m->n, m_cuenta_resh(m), m->dichos, NYX_ACC[m_mejor_accion(m)]);
    }
    tenue(); printf("  detalle: mente <nombre>\n\n"); normal();
}

static void detalle_mente(int r) {
    const Mente *m = &nyx.m[r];
    int i, a, orden[24], n = 0;
    float vmax = 0.01f;
    printf("\n  ");
    negrita(); nombre_rol(r, 0); normal();
    tenue(); printf("  —  quiere: %s\n", NYX_ROLES[r].objetivo); normal();
    printf("  precisión %d%% · %d ideas · sabe %d palabras en Resh (aprendió %ld de las demás)\n",
           (int)(m_precision(m) * 100 + 0.5f), m->n, m_cuenta_resh(m), m->reshAprendidas);
    printf("  dijo %ld cosas · imaginó %ld ideas · cristalizó %ld conceptos · %ld ciclos de pensamiento\n\n",
           m->dichos, m->ideas, m->cristales, m->ciclos);
    tenue(); printf("  lo que le gusta hacer (gusto de su rol + lo que aprendió):\n"); normal();
    for (a = 0; a < NACC; a++) {
        float v = NYX_ROLES[r].gusto[a] + m->valor[a];
        if (v > vmax) vmax = v;
    }
    for (a = 0; a < NACC; a++) {
        float v = NYX_ROLES[r].gusto[a] + m->valor[a];
        printf("    %-9s %5.2f ", NYX_ACC[a], v);
        barra(v > 0 ? v : 0, vmax, 20, NYX_ROLES[r].color);
        tenue(); printf("  (%d veces)\n", m->veces[a]); normal();
    }
    /* lo más activo de su mente */
    for (i = 0; i < m->n; i++) {
        int k, pos;
        if (m->s[i].fusion && m->s[i].dialogo == 0) continue;
        if (nyx_vacia(m->s[i].et)) continue;
        if (n < 12) pos = n++;                                   /* todavía hay sitio */
        else if (m->s[i].A > m->s[orden[11]].A) pos = 11;        /* entra en lugar del último */
        else continue;
        orden[pos] = i;
        for (k = pos; k > 0 && m->s[orden[k]].A > m->s[orden[k - 1]].A; k--) {
            int t = orden[k]; orden[k] = orden[k - 1]; orden[k - 1] = t;
        }
    }
    tenue(); printf("\n  lo más activo en su mente:\n"); normal();
    for (i = 0; i < n; i++) {
        const Semion *s = &m->s[orden[i]];
        const char *rr = s->sabe ? nyx_a_resh(s->et) : NULL;
        printf("    %-24s A=%.2f  C=%+.2f", s->et, s->A, m_coh(m, orden[i]));
        if (rr) { tenue(); printf("  resh: %s", rr); normal(); }
        printf("\n");
    }
    if (m->nfoco > 0) {
        tenue(); printf("\n  en su foco ahora: "); normal();
        for (i = m->nfoco - 1; i >= 0; i--) printf("%s%s", m->s[m->foco[i]].et, i ? " · " : "\n");
    }
    if (m->nven > 0) {
        tenue(); printf("\n  lo último que vivió:\n"); normal();
        for (i = m->nven < 5 ? m->nven : 5; i > 0; i--) {
            int k = (m->iven + NVEN - i) % NVEN;
            printf("    %s\n", m->ven[k]);
        }
    }
    printf("\n");
}

static void muestra_respuesta(const NyxRespuesta *r) {
    int i;
    printf("\n  ");
    color(219); negrita(); printf("nyx: "); normal();
    negrita(); printf("%s\n", r->frase); normal();
    printf("  "); tenue(); printf("resh: %s\n", r->resh); normal();
    printf("  "); tenue();
    printf("ganó «%s» · acuerdo %d%% · habló ", r->ganador, (int)(r->acuerdo * 100 + 0.5f));
    normal();
    if (r->vocero >= 0) nombre_rol(r->vocero, 0);
    printf("\n\n");
    for (i = 0; i < r->nvotos && i < 6; i++) {
        const NyxVoto *v = &r->votos[i];
        printf("   ");
        nombre_rol(v->rol, 11);
        printf(" %s %-18s ", v->acuerdo ? "✓" : " ", v->atractor);
        barra(v->peso, r->votos[0].peso > 0 ? r->votos[0].peso : 1, 12, NYX_ROLES[v->rol].color);
        tenue(); printf("  C=%+.2f E=%.2f\n", v->C, v->E); normal();
    }
    tenue(); printf("   (¿te gustó?  bien / mal)\n\n"); normal();
}

static void ayuda(void) {
    printf("\n");
    negrita(); printf("  Hablar\n"); normal();
    printf("    <pregunta>             las 18 deliberan y responde la que más convence\n");
    printf("    @<mente> <texto>       hablas con una sola (ej: @creativo el mar)\n");
    printf("    bien · mal             tu opinión de la última respuesta (las cambia)\n");
    negrita(); printf("  Verlas vivir (en tiempo real)\n"); normal();
    printf("    vivo [n]               n pasos de vida libre: imaginan, hablan y aprenden\n");
    printf("    conversa <a> <b> [n]   dos mentes conversan n turnos\n");
    printf("    piensa [n]             todas piensan en silencio n ciclos\n");
    negrita(); printf("  Enseñarles\n"); normal();
    printf("    enseña <texto>         las 18 lo leen y lo recuerdan\n");
    printf("    lee <archivo.txt>      leen un archivo entero, línea por línea\n");
    printf("    resh <palabra>         la palabra en Resh (y quién la sabe)\n");
    negrita(); printf("  Mirar por dentro\n"); normal();
    printf("    mentes                 las 18: precisión, ideas, Resh y gustos\n");
    printf("    mente <nombre>         una por dentro: gustos aprendidos, foco, recuerdos\n");
    printf("    eventos si|no          ver o no cuándo aprenden ('ahora vale…')\n");
    negrita(); printf("  Otros\n"); normal();
    printf("    guarda · carga         memoria en %s (se guarda sola al salir)\n", RUTA_MEMORIA);
    printf("    olvida                 empezar de cero (otra infancia)\n");
    printf("    color si|no · salir\n\n");
    tenue(); printf("  mentes: "); normal();
    {
        int r;
        for (r = 0; r < NROLES; r++) { nombre_rol(r, 0); printf(r + 1 < NROLES ? " " : "\n\n"); }
    }
}

/* ------------------------------------------------------------------ */
/*  Órdenes                                                            */
/* ------------------------------------------------------------------ */

static void quita_saltos(char *s) {
    size_t n = strlen(s);
    while (n > 0 && (s[n - 1] == '\n' || s[n - 1] == '\r' || s[n - 1] == ' ' || s[n - 1] == '\t')) s[--n] = 0;
}

static const char *salta_espacios(const char *s) {
    while (*s == ' ' || *s == '\t') s++;
    return s;
}

static int empieza(const char *s, const char *orden, const char **resto) {
    size_t n = strlen(orden);
    if (strncmp(s, orden, n) != 0) return 0;
    if (s[n] != 0 && s[n] != ' ') return 0;
    *resto = salta_espacios(s + n);
    return 1;
}

static int numero(const char *s, int defecto, int max) {
    int n = atoi(s);
    if (n <= 0) n = defecto;
    return n > max ? max : n;
}

static void guarda(int avisa) {
    if (consejo_guarda(&nyx, RUTA_MEMORIA)) {
        if (avisa) { tenue(); printf("  memoria guardada en %s\n", RUTA_MEMORIA); normal(); }
    } else {
        color(203); printf("  no pude guardar la memoria (¿la carpeta no deja escribir?)\n"); normal();
    }
}

static void vivo(int n) {
    int i;
    tenue(); printf("  %d pasos de vida libre — cada línea es lo que hace una mente\n\n", n); normal();
    for (i = 0; i < n; i++) consejo_paso(&nyx);
    printf("\n");
}

/* Dos mentes conversan: cada una responde a lo último que dijo la otra. */
static void conversa(int a, int b, int n) {
    char ultimo[400], es[400], resh[400], linea[900];
    int i;
    const Mente *ma = &nyx.m[a];
    int tema = c_tema(ma);
    nyx_copia(ultimo, tema >= 0 ? ma->s[tema].et : NYX_ROLES[a].objetivo, sizeof ultimo);
    tenue(); printf("  "); normal();
    nombre_rol(a, 0); tenue(); printf(" y "); normal(); nombre_rol(b, 0);
    tenue(); printf(" conversan sobre «%s»\n\n", ultimo); normal();
    for (i = 0; i < n; i++) {
        int quien = i % 2 == 0 ? a : b, otro = quien == a ? b : a, aprend = 0;
        float C;
        char palabra[LARGO];
        nyx.tick++;
        consejo_habla_con(&nyx, quien, ultimo, es, sizeof es, resh, sizeof resh, &C);
        snprintf(linea, sizeof linea, "%s%s   «%s»", i == 0 ? "" : (C < 0.2f ? "ye " : "se ko "), resh, es);
        al_evento(NULL, quien, EV_DICE, linea);
        /* la otra (y las que escuchan) lo oyen */
        if (nyx.fbNfrase > 0) c_difunde(&nyx, quien, nyx.fbFrase, nyx.fbNfrase, &aprend, palabra);
        if (aprend > 0) {
            snprintf(linea, sizeof linea, "%d mente%s aprendi%s «%s» = %s", aprend, aprend > 1 ? "s" : "",
                     aprend > 1 ? "eron" : "ó", palabra, nyx_a_resh(palabra));
            al_evento(NULL, quien, EV_RESH, linea);
        }
        nyx_copia(ultimo, es, sizeof ultimo);
        (void)otro;
    }
    nyx.fbValido = 0;
    printf("\n");
}

static void lee_archivo(const char *ruta) {
    FILE *f = fopen(ruta, "r");
    char l[1024];
    int lineas = 0, nuevas = 0;
    if (!f) { color(203); printf("  no encuentro %s\n", ruta); normal(); return; }
    while (fgets(l, sizeof l, f)) {
        quita_saltos(l);
        if (!*salta_espacios(l)) continue;
        nuevas += consejo_lee(&nyx, l);
        lineas++;
    }
    fclose(f);
    tenue(); printf("  las 18 leyeron %d líneas (%d palabras nuevas)\n", lineas, nuevas); normal();
}

static void resh(const char *palabra) {
    char tok[1][LARGO];
    const char *r, *e;
    int n = nyx_tokens(palabra, tok, 1), q, saben = 0;
    if (n == 0) { printf("  uso: resh <palabra>\n"); return; }
    r = nyx_a_resh(tok[0]);
    e = nyx_a_es(tok[0]);
    if (r) printf("  %s → %s\n", tok[0], r);
    if (e) printf("  %s ← %s (en español)\n", tok[0], e);
    if (!r && !e) { printf("  «%s» no está en el léxico Resh\n", tok[0]); return; }
    for (q = 0; q < NROLES; q++) {
        int id = m_busca(&nyx.m[q], r ? tok[0] : e);
        if (id >= 0 && nyx.m[q].s[id].sabe) saben++;
    }
    tenue(); printf("  la saben decir %d de las 18\n", saben); normal();
}

int main(void) {
    char l[1024];
    int ordenes = 0;
    setvbuf(stdout, NULL, _IONBF, 0);   /* que se vea todo al momento */
    printf("despertando a las 18 mentes…\n");
    nyxp = (Consejo *)calloc(1, sizeof *nyxp);
    if (!nyxp) {
        printf("no hay memoria para las 18 mentes (%lu MB)\n", (unsigned long)(sizeof *nyxp >> 20));
        return 1;
    }
    nyx.ev = al_evento;
    consejo_init(&nyx, (unsigned long long)time(NULL) * 2654435761ULL, 0);
    nyx.ev = al_evento;
    if (consejo_carga(&nyx, RUTA_MEMORIA)) {
        nyx.ev = al_evento;
        banner();
        tenue(); printf("  recordaron todo lo de la última vez (%s)\n\n", RUTA_MEMORIA); normal();
    } else {
        consejo_init(&nyx, (unsigned long long)time(NULL) * 2654435761ULL, 1);
        nyx.ev = al_evento;
        banner();
        tenue(); printf("  nacieron y leyeron su infancia (%d frases)\n\n", NYX_NCORPUS); normal();
    }
    for (;;) {
        const char *s, *resto;
        color(219); negrita(); printf("nyx› "); normal();
        fflush(stdout);
        if (!fgets(l, sizeof l, stdin)) break;
        quita_saltos(l);
        s = salta_espacios(l);
        if (!*s) continue;
        ordenes++;
        if (strcmp(s, "salir") == 0 || strcmp(s, "exit") == 0 || strcmp(s, "q") == 0) break;
        else if (strcmp(s, "ayuda") == 0 || strcmp(s, "help") == 0 || strcmp(s, "?") == 0) ayuda();
        else if (strcmp(s, "mentes") == 0) panel_mentes();
        else if (empieza(s, "mente", &resto)) {
            int r = nyx_rol_de(resto);
            if (r < 0) printf("  ¿qué mente? (escribe: mentes)\n"); else detalle_mente(r);
        }
        else if (empieza(s, "vivo", &resto)) { vivo(numero(resto, 20, 2000)); guarda(0); }
        else if (empieza(s, "conversa", &resto)) {
            char a[32] = "", b[32] = "";
            int n = 6, r1, r2;
            if (sscanf(resto, "%31s %31s %d", a, b, &n) < 2) { printf("  uso: conversa <mente> <mente> [turnos]\n"); continue; }
            r1 = nyx_rol_de(a); r2 = nyx_rol_de(b);
            if (r1 < 0 || r2 < 0 || r1 == r2) { printf("  ¿qué mentes? (escribe: mentes)\n"); continue; }
            conversa(r1, r2, n < 1 ? 1 : (n > 200 ? 200 : n));
        }
        else if (empieza(s, "piensa", &resto)) {
            int n = numero(resto, 50, 5000), i, r;
            for (i = 0; i < n; i++) for (r = 0; r < NROLES; r++) m_piensa(&nyx.m[r]);
            tenue(); printf("  pensaron %d ciclos en silencio\n", n); normal();
        }
        else if (empieza(s, "enseña", &resto) || empieza(s, "ensena", &resto)) {
            if (!*resto) { printf("  uso: enseña <texto>\n"); continue; }
            tenue(); printf("  las 18 lo leyeron (%d palabras nuevas)\n", consejo_lee(&nyx, resto)); normal();
        }
        else if (empieza(s, "lee", &resto)) lee_archivo(resto);
        else if (empieza(s, "resh", &resto)) resh(resto);
        else if (strcmp(s, "bien") == 0 || strcmp(s, "mal") == 0) {
            int bueno = s[0] == 'b';
            if (consejo_opina(&nyx, bueno)) {
                tenue();
                printf(bueno ? "  lo reforzaron: esa idea y esa frase pesan más ahora\n"
                             : "  lo inhibieron: esa respuesta queda en oposición de fase y perderá la próxima vez\n");
                normal();
            } else printf("  no hay respuesta reciente que juzgar\n");
        }
        else if (empieza(s, "eventos", &resto)) { verAprende = strcmp(resto, "no") != 0; }
        else if (empieza(s, "color", &resto)) { conColor = strcmp(resto, "no") != 0; }
        else if (strcmp(s, "guarda") == 0) guarda(1);
        else if (strcmp(s, "carga") == 0) {
            if (consejo_carga(&nyx, RUTA_MEMORIA)) { nyx.ev = al_evento; printf("  memoria cargada\n"); }
            else printf("  no hay memoria guardada (o está dañada)\n");
        }
        else if (strcmp(s, "olvida") == 0) {
            consejo_init(&nyx, (unsigned long long)time(NULL) * 2654435761ULL, 1);
            nyx.ev = al_evento;
            printf("  empezaron de cero (otra infancia)\n");
        }
        else if (s[0] == '@') {
            char nom[32] = "", es[400], rs[400];
            float C;
            int r, k = 0;
            const char *p = s + 1;
            while (*p && *p != ' ' && k < 31) nom[k++] = *p++;
            nom[k] = 0;
            r = nyx_rol_de(nom);
            if (r < 0) { printf("  ¿qué mente? (escribe: mentes)\n"); continue; }
            p = salta_espacios(p);
            if (!*p) { printf("  uso: @%s <texto>\n", nom); continue; }
            consejo_habla_con(&nyx, r, p, es, sizeof es, rs, sizeof rs, &C);
            printf("\n  "); nombre_rol(r, 0); printf(": "); negrita(); printf("%s\n", es); normal();
            printf("  "); tenue(); printf("resh: %s%s   (C=%+.2f · ¿bien / mal?)\n\n", C < 0.2f ? "ye " : "mi ko ", rs, C); normal();
        }
        else {
            NyxRespuesta r;
            consejo_delibera(&nyx, s, &r);
            muestra_respuesta(&r);
        }
        if (ordenes % 15 == 0) guarda(0);
    }
    guarda(1);
    printf("  hasta luego\n");
    return 0;
}
