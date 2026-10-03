// Prueba mínima: ¿Code App ejecuta programas C++?
//   clang++ -std=c++17 diagnostico/hola.cpp -o hola && wasm hola
#include <cstdio>

int main() {
  std::printf("Hola desde C++. Si lees esto, Code App ejecuta programas.\n");
  return 0;
}
