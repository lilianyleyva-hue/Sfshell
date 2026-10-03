/* memoria.c — guardar y cargar lo aprendido */
#include "nyx.h"

/* ------------------------------------------------------------------ */
/*  Memoria en disco (texto)                                           */
/* ------------------------------------------------------------------ */

int consejo_guarda(const Consejo *c, const char *ruta) {
    FILE *f = fopen(ruta, "w");
    int r, i, k;
    if (!f) return 0;
    fprintf(f, "NYX 2 %lu\n", c->tick);
    for (r = 0; r < NROLES; r++) {
        const Mente *m = &c->m[r];
        fprintf(f, "MENTE %d %d %d %d %ld %ld %ld %ld %ld\n", r, m->n, m->aciertos, m->intentos,
                m->ciclos, m->dichos, m->ideas, m->cristales, m->reshAprendidas);
        fprintf(f, "VAL");
        for (i = 0; i < NACC; i++) fprintf(f, " %.4f %d", m->valor[i], m->veces[i]);
        fprintf(f, "\nOBJ %d", m->nobj);
        for (i = 0; i < m->nobj; i++) fprintf(f, " %d", m->obj[i]);
        fprintf(f, "\n");
        for (i = 0; i < m->n; i++) {
            const Semion *s = &m->s[i];
            fprintf(f, "S %s %.4f %.4f %.4f %d %d %d %u %u", s->et, s->A, s->fase, s->frec,
                    s->carga, s->fusion, s->sabe, s->usos, s->dialogo);
            for (k = 0; k < DIMF; k++) fprintf(f, " %.4f", s->f[k]);
            fprintf(f, " %d", s->nv);
            for (k = 0; k < s->nv; k++) fprintf(f, " %d %.4f %.4f %.4f", s->v[k].j, s->v[k].w, s->v[k].th, s->v[k].sec);
            fprintf(f, "\n");
        }
    }
    fprintf(f, "FRASES %d\n", c->nfr);
    for (r = 0; r < c->nfr; r++) fprintf(f, "F %.3f %u %s\n", c->fr[r].puntos, c->fr[r].usos, c->fr[r].t);
    fprintf(f, "FIN\n");
    return fclose(f) == 0;
}


int consejo_carga(Consejo *c, const char *ruta) {
    /* se carga aparte (en memoria pedida al momento): si el archivo está
     * mal, no se pierde nada */
    FILE *f = fopen(ruta, "r");
    Consejo *tmp;
    int ok;
    if (!f) return 0;
    tmp = (Consejo *)malloc(sizeof *tmp);
    if (!tmp) { fclose(f); return 0; }
    *tmp = *c;
    ok = consejo_carga2(c, tmp, f);
    free(tmp);
    return ok;
}

int consejo_carga2(Consejo *c, Consejo *tmp0, FILE *f) {
    int r, i, k, ver;
    char cab[8];
    if (fscanf(f, "%7s %d %lu", cab, &ver, &tmp0->tick) != 3 || strcmp(cab, "NYX") != 0) { fclose(f); return 0; }
    for (r = 0; r < NROLES; r++) {
        Mente *m = &tmp0->m[r];
        int rr, n, nobj;
        if (fscanf(f, " MENTE %d %d %d %d %ld %ld %ld %ld %ld", &rr, &n, &m->aciertos, &m->intentos,
                   &m->ciclos, &m->dichos, &m->ideas, &m->cristales, &m->reshAprendidas) != 9 || rr != r || n < 0 || n > MAXSEM) { fclose(f); return 0; }
        if (fscanf(f, " VAL") != 0) { fclose(f); return 0; }
        for (i = 0; i < NACC; i++) if (fscanf(f, " %f %d", &m->valor[i], &m->veces[i]) != 2) { fclose(f); return 0; }
        if (fscanf(f, " OBJ %d", &nobj) != 1 || nobj < 0 || nobj > 8) { fclose(f); return 0; }
        m->nobj = nobj;
        for (i = 0; i < nobj; i++) { int x; if (fscanf(f, " %d", &x) != 1 || x < 0 || x >= n) { fclose(f); return 0; } m->obj[i] = (short)x; }
        m->n = n;
        for (i = 0; i < n; i++) {
            Semion *s = &m->s[i];
            int carga, fusion, sabe, nv;
            unsigned usos, dialogo;
            memset(s, 0, sizeof *s);
            if (fscanf(f, " S %27s %f %f %f %d %d %d %u %u", s->et, &s->A, &s->fase, &s->frec,
                       &carga, &fusion, &sabe, &usos, &dialogo) != 9) { fclose(f); return 0; }
            s->carga = (signed char)carga; s->fusion = (unsigned char)fusion; s->sabe = (unsigned char)sabe;
            s->usos = (unsigned short)usos; s->dialogo = (unsigned short)dialogo;
            for (k = 0; k < DIMF; k++) if (fscanf(f, " %f", &s->f[k]) != 1) { fclose(f); return 0; }
            if (fscanf(f, " %d", &nv) != 1 || nv < 0 || nv > MAXVEC) { fclose(f); return 0; }
            s->nv = (unsigned char)nv;
            for (k = 0; k < nv; k++) {
                int j;
                if (fscanf(f, " %d %f %f %f", &j, &s->v[k].w, &s->v[k].th, &s->v[k].sec) != 4 || j < 0 || j >= n) { fclose(f); return 0; }
                s->v[k].j = (short)j;
            }
        }
        m->nfoco = 0; m->nepis = 0; m->ncam = 0; m->nven = 0; m->iven = 0; m->nrec = 0;
        m->ultima = -1; m->racha = 0;
        m_hash_rehace(m);
    }
    {   /* frases recordadas (los archivos viejos no las tienen) */
        char pal[16];
        tmp0->nfr = 0;
        if (fscanf(f, " %15s", pal) == 1 && strcmp(pal, "FRASES") == 0) {
            int nf, q;
            if (fscanf(f, " %d", &nf) != 1 || nf < 0 || nf > MAXFR) { fclose(f); return 0; }
            for (q = 0; q < nf; q++) {
                FraseMem *x = &tmp0->fr[q];
                unsigned u;
                char lin[LFRASE + 8];
                if (fscanf(f, " F %f %u ", &x->puntos, &u) != 2 || !fgets(lin, sizeof lin, f)) { fclose(f); return 0; }
                lin[strcspn(lin, "\r\n")] = 0;
                nyx_copia(x->t, lin, LFRASE);
                x->usos = (unsigned short)u;
            }
            tmp0->nfr = nf;
        } else {
            int q;
            for (q = 0; q < NYX_NCORPUS; q++) frases_pon(tmp0, NYX_CORPUS[q]);
        }
        tmp0->fbFr = -1;
    }
    fclose(f);
    *c = *tmp0;
    return 1;
}

/* Cuántas palabras sabe decir en Resh una mente. */
int m_cuenta_resh(const Mente *m) {
    int i, n = 0;
    for (i = 0; i < m->n; i++) if (m->s[i].sabe) n++;
    return n;
}

int m_mejor_accion(const Mente *m) {
    int a, mejor = 0;
    for (a = 1; a < NACC; a++)
        if (NYX_ROLES[m->rol].gusto[a] + m->valor[a] > NYX_ROLES[m->rol].gusto[mejor] + m->valor[mejor]) mejor = a;
    return mejor;
}
