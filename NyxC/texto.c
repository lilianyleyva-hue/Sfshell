/* texto.c — palabras en minúscula y firma formal */
#include "nyx.h"

/* ------------------------------------------------------------------ */
/*  Texto: palabras en minúscula, sin puntuación                       */
/* ------------------------------------------------------------------ */

int nyx_tokens(const char *t, char toks[][LARGO], int max) {
    int n = 0;
    char w[64];
    int lw = 0;
    const unsigned char *p = (const unsigned char *)t;
    for (;;) {
        unsigned char c = *p;
        int corta = 0;
        if (c == 0) corta = 1;
        else if (c < 0x80) {
            if ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
                if (lw < 60) w[lw++] = (char)c;
            } else if (c >= 'A' && c <= 'Z') {
                if (lw < 60) w[lw++] = (char)(c + 32);
            } else corta = 1;
            p++;
        } else if (c == 0xE2 && p[1] && p[2]) {
            corta = 1;            /* ◀ ▶ ⊕ … — símbolos, no letras */
            p += 3;
        } else if (c == 0xC2 && (p[1] == 0xBF || p[1] == 0xA1 || p[1] == 0xAB || p[1] == 0xBB)) {
            corta = 1;            /* ¿ ¡ « » */
            p += 2;
        } else if (c == 0xC3 && p[1] >= 0x80 && p[1] <= 0x9E && p[1] != 0x97) {
            if (lw < 59) { w[lw++] = (char)c; w[lw++] = (char)(p[1] + 0x20); }  /* Á -> á */
            p += 2;
        } else {
            if (lw < 60) w[lw++] = (char)c;
            p++;
        }
        if (corta || lw >= 60) {
            if (lw > 0 && n < max) {
                w[lw] = 0;
                nyx_copia(toks[n++], w, LARGO);
            }
            lw = 0;
            if (c == 0) break;
        }
    }
    return n;
}

/* Firma formal de una palabra (sin embeddings): forma, ritmo y símbolos. */
void nyx_firma(const char *w, float *f) {
    int len = 0, nb = 0, sim = 0, dig = 0, voc = 0, simb = 0, acen = 0, i;
    double suma = 0, var = 0, media;
    int L = (int)strlen(w);
    for (i = 0; i < L; i++) {
        unsigned char c = (unsigned char)w[i];
        if ((c & 0xC0) != 0x80) len++;
        suma += c; nb++;
        if (c >= '0' && c <= '9') dig++;
        if (strchr("aeiou", c) && c) voc++;
        if (strchr("+-*/=<>^", c) && c) simb++;
        if (c >= 0x80) acen++;
    }
    media = nb ? suma / nb : 0;
    for (i = 0; i < L; i++) { double d = (unsigned char)w[i] - media; var += d * d; }
    var = nb ? var / nb : 0;
    for (i = 0; i < L / 2; i++) if (w[i] == w[L - 1 - i]) sim++;
    if (len == 0) len = 1;
    f[0] = (float)len / 20.0f;
    f[1] = (float)fmod(media, 128.0) / 128.0f;
    f[2] = nyx_min(1.0f, (float)(var / 4000.0));
    f[3] = (float)sim / (float)len;
    f[4] = (float)dig / (float)len;
    f[5] = (float)acen / (float)(nb ? nb : 1);
    f[6] = (float)voc / (float)len;
    f[7] = (float)simb / (float)len;
}
