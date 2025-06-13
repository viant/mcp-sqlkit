package main

import (
	_ "github.com/viant/mcp-sqlkit/db/driver"
	"log"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
