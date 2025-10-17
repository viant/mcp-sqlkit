package main

import (
	"log"
	"os"

	_ "github.com/viant/mcp-sqlkit/db/driver"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
