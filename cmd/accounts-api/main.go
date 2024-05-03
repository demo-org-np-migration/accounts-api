// Command accounts-api serves the read-side of customer accounts. See
// docs/adr/ADR-003-lectura-directa.md for why it talks to the ledger
// database directly instead of going through ledger-core.
package main

func main() {
	run()
}
