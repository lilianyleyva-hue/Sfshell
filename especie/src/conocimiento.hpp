#pragma once
// Conocimiento de un ser: hechos "sujeto —relación→ objeto".
//
// Se guarda como C++ de verdad: conocimiento/<Nombre>.cpp es un arreglo
// `const Hecho Nombre[] = {...}` que compila con cualquier compilador, y al
// despertar el ser vuelve a leer su conocimiento de ese mismo archivo.

#include <cstdint>
#include <cstdio>
#include <filesystem>
#include <fstream>
#include <functional>
#include <string>
#include <unordered_map>
#include <vector>

#include "hecho.hpp"

namespace abla {

namespace fs = std::filesystem;

constexpr uint32_t FUENTE_SEMILLA = 100;  // innato: nació sabiéndolo
constexpr uint32_t FUENTE_HUMANO = 200;   // se lo enseñó un humano

inline const char* HECHO_HPP =
    "#pragma once\n"
    "#include <cstdint>\n"
    "// Un hecho: sujeto -relacion-> objeto. Sujeto y objeto son ids de palabras de Abla\n"
    "// (ver ../abla/diccionario.txt). Relaciones: 0 es, 1 parte de, 2 causa, 3 igual a, 4 opuesto a.\n"
    "// fuente: 0-26 = otro ser, 100 = innato, 200 = humano.\n"
    "struct Hecho {\n"
    "  std::uint16_t s;\n"
    "  std::uint8_t r;\n"
    "  std::uint16_t o;\n"
    "  float confianza;\n"
    "  std::uint32_t fuente;\n"
    "};\n";

class Conocimiento {
 public:
  static constexpr size_t MAX_HECHOS = 50000;

  // Devuelve true si el hecho es nuevo. Si ya lo sabía, refuerza la confianza.
  bool agregar(Hecho h) {
    if (h.s == h.o || h.r >= 5) return false;
    auto k = clave(h.s, h.r, h.o);
    if (auto it = indice_.find(k); it != indice_.end()) {
      auto& v = hechos_[it->second];
      if (h.confianza > v.confianza) v.confianza = h.confianza;
      return false;
    }
    if (hechos_.size() >= MAX_HECHOS) return false;
    indice_[k] = hechos_.size();
    porSujeto_[h.s].push_back(hechos_.size());
    hechos_.push_back(h);
    return true;
  }

  const Hecho* buscar(int s, int r, int o) const {
    auto it = indice_.find(clave(s, r, o));
    return it == indice_.end() ? nullptr : &hechos_[it->second];
  }
  Hecho* buscar(int s, int r, int o) {
    auto it = indice_.find(clave(s, r, o));
    return it == indice_.end() ? nullptr : &hechos_[it->second];
  }

  std::vector<const Hecho*> desde(int s, int r) const {
    std::vector<const Hecho*> v;
    if (auto it = porSujeto_.find(s); it != porSujeto_.end())
      for (size_t i : it->second)
        if (hechos_[i].r == r) v.push_back(&hechos_[i]);
    return v;
  }

  const std::vector<Hecho>& todos() const { return hechos_; }
  size_t size() const { return hechos_.size(); }
  bool lleno() const { return hechos_.size() >= MAX_HECHOS; }

  void exportarCpp(const fs::path& archivo, const std::string& nombre, const std::string& encabezado,
                   const std::function<std::string(const Hecho&)>& comentario) const {
    fs::path tmp = archivo;
    tmp += ".tmp";
    {
      std::ofstream out(tmp);
      out << "// " << encabezado << "\n"
          << "// Este archivo es C++ válido y es, a la vez, la memoria de este ser:\n"
          << "// al despertar vuelve a leer de aquí todo lo que sabe.\n"
          << "#include \"hecho.hpp\"\n\nnamespace conocimiento {\n\n"
          << "const Hecho " << nombre << "[] = {\n";
      if (hechos_.empty()) out << "    {0, 0, 0, 0.000f, 0},  // (vacío)\n";
      char buf[96];
      for (const auto& h : hechos_) {
        std::snprintf(buf, sizeof buf, "    {%5u, %u, %5u, %.3ff, %3u},", h.s, h.r, h.o, h.confianza, h.fuente);
        out << buf << "  // " << comentario(h) << "\n";
      }
      out << "};\n\nconst unsigned " << nombre << "_n = sizeof(" << nombre << ") / sizeof(" << nombre
          << "[0]);\n\n}  // namespace conocimiento\n";
    }
    fs::rename(tmp, archivo);
  }

  bool importarCpp(const fs::path& archivo, int maxPalabra) {
    std::ifstream in(archivo);
    if (!in) return false;
    std::string linea;
    while (std::getline(in, linea)) {
      unsigned s, r, o, f;
      float c;
      auto p = linea.find('{');
      if (p == std::string::npos) continue;
      if (std::sscanf(linea.c_str() + p, " { %u , %u , %u , %ff , %u }", &s, &r, &o, &c, &f) == 5 &&
          c > 0 && static_cast<int>(s) < maxPalabra && static_cast<int>(o) < maxPalabra)
        agregar({static_cast<uint16_t>(s), static_cast<uint8_t>(r), static_cast<uint16_t>(o), c, f});
    }
    return true;
  }

 private:
  static uint64_t clave(uint64_t s, uint64_t r, uint64_t o) { return (s << 24) | (r << 20) | o; }

  std::vector<Hecho> hechos_;
  std::unordered_map<uint64_t, size_t> indice_;
  std::unordered_map<int, std::vector<size_t>> porSujeto_;
};

}  // namespace abla
