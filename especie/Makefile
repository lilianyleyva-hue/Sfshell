CXX ?= g++
CXXFLAGS ?= -std=c++17 -O2 -Wall -Wextra -pthread

abla: src/*.hpp src/main.cpp
	$(CXX) $(CXXFLAGS) src/main.cpp -o abla

clean:
	rm -f abla

.PHONY: clean
