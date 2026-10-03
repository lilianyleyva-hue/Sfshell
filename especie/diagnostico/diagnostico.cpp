// Diagnóstico paso a paso: muestra hasta dónde llega en este entorno.
//   clang++ -std=c++17 diagnostico/diagnostico.cpp -o diag && wasm diag
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <iostream>
#include <map>
#include <string>
#include <vector>

#include "../src/archivos.hpp"
#include "../src/lenguaje.hpp"

static void paso(int n, const char* que) {
  std::printf("[%d] %s... ", n, que);
  std::fflush(stdout);
}
static void ok() {
  std::printf("ok\n");
  std::fflush(stdout);
}

int main() {
  std::printf("Diagnóstico de la Especie Abla\n");
  std::fflush(stdout);

  paso(1, "std::cout");
  std::cout << "(cout funciona) " << std::flush;
  ok();

  paso(2, "memoria: string, vector, map");
  std::vector<std::string> v(5000, "abcdefgh");
  std::map<int, std::string> m;
  for (int i = 0; i < 5000; i++) m[i] = v[i];
  ok();

  paso(3, "reloj");
  auto t = std::chrono::steady_clock::now().time_since_epoch().count();
  std::printf("(%lld) ", static_cast<long long>(t % 100000));
  ok();

  paso(4, "crear carpeta diag_mundo/a/b");
  bool c = abla::archivos::crearDirectorios("diag_mundo/a/b");
  std::printf("%s ", c ? "(creada)" : "(FALLÓ)");
  ok();

  paso(5, "escribir y leer un archivo");
  bool w = abla::archivos::escribir("diag_mundo/a/b/x.txt", "hola\n");
  std::string leido;
  bool r = abla::archivos::leer("diag_mundo/a/b/x.txt", leido);
  std::printf("(escribir %s, leer %s) ", w ? "ok" : "FALLÓ", r && leido == "hola\n" ? "ok" : "FALLÓ");
  ok();

  paso(6, "listar carpeta");
  auto e = abla::archivos::listar("diag_mundo/a");
  std::printf("(%zu entradas) ", e.size());
  ok();

  paso(7, "borrar carpeta");
  bool b = abla::archivos::borrarTodo("diag_mundo");
  std::printf("%s ", b ? "(borrada)" : "(FALLÓ)");
  ok();

  paso(8, "crear el idioma Abla (4000 palabras)");
  abla::Idioma idioma;
  std::printf("(%d palabras, palabra = %s) ", idioma.tamano(), idioma.forma(idioma.deEspanol("palabra")).c_str());
  ok();

  paso(9, "leer lo que escribes (escribe algo y pulsa return)");
  std::string linea;
  if (std::getline(std::cin, linea)) std::printf("(leí: %s) ", linea.c_str());
  else std::printf("(no hay entrada: stdin cerrado) ");
  ok();

  std::printf("\nTodo funciona. Si abla no arranca, mándale esta salida a Claude.\n");
  return 0;
}
