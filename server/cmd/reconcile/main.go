// Command reconcile is the one-shot startup task that syncs data/vehicles.json into
// Postgres (insert-only-new) and primes Redis listing state (see
// guidelines/06-backend-architecture.md, "Data lifecycle"). Real logic lands in a later commit.
package main

import "fmt"

func main() {
	fmt.Println("the-block reconcile: under construction, see guidelines/06-backend-architecture.md")
}
