/* resh.c — diccionario de la lengua Resh (tabla hash sobre resh.h) */
#include "nyx.h"
#include "resh.h"

#define NYX_DICN 8192

/* ------------------------------------------------------------------ */
/*  Diccionario Resh (tabla hash sobre resh.h)                         */
/* ------------------------------------------------------------------ */

static short nyx_dic_es[NYX_DICN], nyx_dic_re[NYX_DICN];
static int nyx_dic_listo = 0;

static const ParResh *nyx_par(int k) {
    return k < N_RESH_PROTOCOLO ? &RESH_PROTOCOLO[k] : &RESH_LEXICO[k - N_RESH_PROTOCOLO];
}

static void nyx_dic_pon(short *t, int k, int porResh) {
    const char *clave = porResh ? nyx_par(k)->resh : nyx_par(k)->es;
    unsigned h = nyx_fnv(clave) & (NYX_DICN - 1);
    while (t[h] >= 0) {
        const ParResh *p = nyx_par(t[h]);
        if (strcmp(porResh ? p->resh : p->es, clave) == 0) return;  /* el primero manda */
        h = (h + 1) & (NYX_DICN - 1);
    }
    t[h] = (short)k;
}

void nyx_dic_init(void) {
    int k;
    if (nyx_dic_listo) return;
    for (k = 0; k < NYX_DICN; k++) { nyx_dic_es[k] = -1; nyx_dic_re[k] = -1; }
    for (k = 0; k < N_RESH_PROTOCOLO + N_RESH_LEXICO; k++) {
        nyx_dic_pon(nyx_dic_es, k, 0);
        nyx_dic_pon(nyx_dic_re, k, 1);
    }
    nyx_dic_listo = 1;
}

static const ParResh *nyx_dic_busca(const short *t, const char *clave, int porResh) {
    unsigned h = nyx_fnv(clave) & (NYX_DICN - 1);
    while (t[h] >= 0) {
        const ParResh *p = nyx_par(t[h]);
        if (strcmp(porResh ? p->resh : p->es, clave) == 0) return p;
        h = (h + 1) & (NYX_DICN - 1);
    }
    return NULL;
}

/* español -> resh (NULL si no está en el léxico) */
const char *nyx_a_resh(const char *es) {
    const ParResh *p = nyx_dic_busca(nyx_dic_es, es, 0);
    return p ? p->resh : NULL;
}

/* resh -> español */
const char *nyx_a_es(const char *resh) {
    const ParResh *p = nyx_dic_busca(nyx_dic_re, resh, 1);
    return p ? p->es : NULL;
}

int nyx_es_particula(const char *es) {
    int k;
    for (k = 0; k < N_RESH_PROTOCOLO; k++)
        if (strcmp(RESH_PROTOCOLO[k].es, es) == 0) return 1;
    return 0;
}
