//go:build !windows

// The miele GUI targets Windows. This stub lets the module build on other platforms;
// use cmd/mielecli for the headless engine during development on Linux/macOS.
package main

import "fmt"

func main() {
	fmt.Println("The miele GUI runs on Windows only. Use 'mielecli' for the command-line engine.")
}
