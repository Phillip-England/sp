package main

import (
	"errors"
	"fmt"
	"os"

	"sp/internal/sp"
)

func main() {
	if err := sp.Run(os.Args[1:]); err != nil {
		if errors.Is(err, sp.ErrUsage) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		fmt.Fprintln(os.Stderr, "sp:", err)
		os.Exit(1)
	}
}
