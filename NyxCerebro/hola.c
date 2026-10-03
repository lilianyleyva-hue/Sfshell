/* hola.c — prueba mínima: si esto no imprime, el problema es Code App, no Nyx */
#include <stdio.h>
#include <stdlib.h>

int main(void) {
    char *p;
    printf("hola desde C\n");
    fflush(stdout);
    p = (char *)malloc(8u << 20);   /* 8 MB, lo que necesita Nyx */
    printf(p ? "memoria: 8 MB ok\n" : "memoria: NO hay 8 MB\n");
    free(p);
    return 0;
}
