package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/driusan/dgit/git"
)

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "\tschemaman [show | import | diff] [subcommand options]\n")
		os.Exit(1)
	}

	c, err := git.NewClient("", "")
	if err != nil {
		log.Fatal(err)
	}
	switch args[0] {
	case "show":
		if err := Show(c, args[1:]); err != nil {
			log.Fatalln(err)
		}
	case "diff":
		if err := Diff(c, args[1:]); err != nil {
			log.Fatalln(err)
		}
	case "import":
		if err := Import(c, args[1:]); err != nil {
			log.Fatalln(err)
		}

	}
}
