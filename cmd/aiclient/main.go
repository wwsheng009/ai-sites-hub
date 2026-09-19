// aiclient 命令行入口（architecture §7：serve / doctor / migrate / version）。
package main

import (
	"fmt"
	"os"

	"aiclient/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}
