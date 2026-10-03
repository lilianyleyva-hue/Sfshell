CC ?= cc
CFLAGS ?= -std=c11 -O2 -Wall -Wextra
SRC := $(wildcard src/*.c)
# la app gráfica (gui*.c y la letra) solo hace falta en la versión web
SRC_TERMINAL := $(filter-out src/gui.c src/gui_dibujo.c src/fuente.c,$(SRC))

# Versión de terminal (Linux, macOS, Termux, iSH)
abla: $(SRC_TERMINAL) src/*.h
	$(CC) $(CFLAGS) $(SRC_TERMINAL) -o abla -lpthread

# Cerebro en WebAssembly para la interfaz web (necesita clang con wasm32-wasi)
# (wasi-sdk: WASI_FLAGS=--sysroot=/ruta/wasi-sysroot · Ubuntu: apt install wasi-libc)
WASI_FLAGS ?= -isystem /usr/include/wasm32-wasi -L/usr/lib/wasm32-wasi
web/abla.wasm: $(SRC) src/*.h
	clang --target=wasm32-wasi $(WASI_FLAGS) -std=c11 -O2 -Wall -mexec-model=reactor -Wl,--strip-all \
	  $(filter-out src/main.c,$(SRC)) -o web/abla.wasm
	node web/empaquetar.mjs

web: web/abla.wasm

clean:
	rm -f abla

.PHONY: web clean
