// quiero: compila
package main

import "fmt"

function main() {
	let nombres = []string{"ana", "luis"}
	for (const n of nombres) {
		console.log(n.toUpperCase())
	}
	console.log(nombres.length)
}
