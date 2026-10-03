#pragma once
// Memoria de un ser.
//
//  - Contexto (memoria de trabajo): los últimos N tokens de recuerdos, en RAM.
//    Por defecto 1.000.000 de tokens por ser.
//  - Largo plazo: TODO lo que el ser ha pensado, en un archivo en disco que
//    solo crece. Nunca olvida; solo deja de tenerlo "presente".
//
// Al despertar, el contexto se reconstruye leyendo la cola del archivo.

#include <algorithm>
#include <cctype>
#include <cstdint>
#include <deque>
#include <cstdlib>
#include <sstream>
#include <string>
#include <vector>

#include "archivos.hpp"

namespace abla {

class Memoria {
 public:
  struct Recuerdo {
    uint64_t tick;
    std::string texto;
    uint32_t tokens;
  };

  void configurar(uint64_t capacidadTokens, const std::string& archivo) {
    capacidad_ = capacidadTokens;
    archivo_ = archivo;
    cargar();
    log_.abrir(archivo_);
  }

  static uint32_t tokens(const std::string& s) {
    uint32_t n = 0;
    bool enPalabra = false;
    for (unsigned char c : s) {
      bool espacio = c == ' ' || c == '\t' || c == '\n';
      if (!espacio && !enPalabra) n++;
      enPalabra = !espacio;
    }
    return n ? n : 1;
  }

  void recordar(uint64_t tick, const std::string& texto) {
    std::string limpio = texto;
    for (auto& c : limpio)
      if (c == '\n' || c == '\t') c = ' ';
    log_.escribir(std::to_string(tick) + '\t' + limpio + '\n');
    totalLargoPlazo_++;
    meter({tick, limpio, tokens(limpio)});
  }

  const std::deque<Recuerdo>& contexto() const { return ventana_; }
  uint64_t usados() const { return usados_; }
  uint64_t capacidad() const { return capacidad_; }
  uint64_t largoPlazo() const { return totalLargoPlazo_; }
  long long bytesEnDisco() const { return std::max(0LL, archivos::tamano(archivo_)); }
  const std::string& archivo() const { return archivo_; }
  void sincronizar() { log_.vaciar(); }

  // Busca primero en el contexto y luego en todo el largo plazo.
  std::vector<std::string> buscar(const std::string& consulta, size_t max) {
    auto bajo = [](std::string x) {
      for (auto& c : x) c = static_cast<char>(std::tolower(static_cast<unsigned char>(c)));
      return x;
    };
    const std::string q = bajo(consulta);
    std::vector<std::string> r;
    for (auto it = ventana_.rbegin(); it != ventana_.rend() && r.size() < max; ++it)
      if (bajo(it->texto).find(q) != std::string::npos)
        r.push_back("[t=" + std::to_string(it->tick) + "] " + it->texto);
    if (r.size() < max && totalLargoPlazo_ > ventana_.size()) {
      log_.vaciar();
      archivos::Lector in(archivo_);
      std::string linea;
      uint64_t enDisco = 0, limite = totalLargoPlazo_ - ventana_.size();
      while (in.linea(linea) && enDisco++ < limite && r.size() < max) {
        auto tab = linea.find('\t');
        if (tab != std::string::npos && bajo(linea).find(q, tab) != std::string::npos)
          r.push_back("[t=" + linea.substr(0, tab) + ", largo plazo] " + linea.substr(tab + 1));
      }
    }
    return r;
  }

 private:
  void meter(Recuerdo r) {
    usados_ += r.tokens;
    ventana_.push_back(std::move(r));
    while (usados_ > capacidad_ && ventana_.size() > 1) {
      usados_ -= ventana_.front().tokens;
      ventana_.pop_front();
    }
  }

  void cargar() {
    archivos::Lector in(archivo_);
    std::string linea;
    while (in.linea(linea)) {
      auto tab = linea.find('\t');
      if (tab == std::string::npos) continue;
      totalLargoPlazo_++;
      std::string texto = linea.substr(tab + 1);
      meter({std::strtoull(linea.c_str(), nullptr, 10), texto, tokens(texto)});
    }
  }

  uint64_t capacidad_ = 1'000'000, usados_ = 0, totalLargoPlazo_ = 0;
  std::deque<Recuerdo> ventana_;
  std::string archivo_;
  archivos::Anexador log_;
};

}  // namespace abla
