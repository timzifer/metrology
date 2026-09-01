// Package unrelated does not reach the library at all. The pass has nothing to
// say about it and stops before it builds any state.
package unrelated

import "fmt"

func Greet() { fmt.Println("no units here") }
