package main

import (
	"encoding/json"
	"flag"
	"github.com/artpar/apigate/internal/contracts"
	"os"
)

func main() {
	ts := flag.Bool("typescript", false, "Generate TypeScript models")
	flag.Parse()
	if *ts {
		_, err := os.Stdout.WriteString(contracts.TypeScript())
		if err != nil {
			panic(err)
		}
		return
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	if err := e.Encode(contracts.Document()); err != nil {
		panic(err)
	}
}
