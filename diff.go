package main

import (
	"fmt"
	//	"strings"
	"log"
	"os/exec"

	"github.com/driusan/dgit/git"
)

func Diff(c *git.Client, args []string) error {
	var from, to string
	switch len(args) {
	case 1:
		args = append(args, "HEAD")
		fallthrough
	case 2:
		from = args[0] + ":.schema/tables"
		to = args[1] + ":.schema/tables"
	default:
		return fmt.Errorf("Invalid usage of schemaman diff")
	}
	cmd := exec.Command("git", "rev-parse", from)
	val, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	if len(val) != 41 {
		// 40 bytes for the hash as a hexadecimal string, 1 for the
		// newline
		log.Fatalf("No valid table schema found.")
	}
	fromsha, err := git.Sha1FromString(string(val[:40]))
	if err != nil {
		log.Fatal(err)
	}
	cmd = exec.Command("git", "rev-parse", to)
	val, err = cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	if len(val) != 41 {
		// 40 bytes for the hash as a hexadecimal string, 1 for the
		// newline
		log.Fatalf("No valid table schema found.")
	}
	tosha, err := git.Sha1FromString(string(val[:40]))
	if err != nil {
		log.Fatal(err)
	}

	diffs, err := git.DiffTree(c, &git.DiffTreeOptions{}, git.TreeID(fromsha), git.TreeID(tosha), nil)
	if err != nil {
		log.Fatal(err)
	}
	return PrintAlters(c, diffs)
}

func PrintAlters(c *git.Client, tables []git.HashDiff) error {
	for _, changedTable := range tables {
		if changedTable.Dst.FileMode == 0 {
			fmt.Printf("DROP TABLE %s;\n", changedTable.Name.String())
		} else if changedTable.Src.FileMode == 0 {
			if err := PrintTable(c, changedTable.Name.String(), changedTable.Dst); err != nil {
				log.Fatalln("ChangedTable:", err)
			}
		} else if err := PrintTableAlters(c, changedTable.Name.String(), git.TreeID(changedTable.Src.Sha1), git.TreeID(changedTable.Dst.Sha1)); err != nil {
			log.Fatalln("AlterTable", err)
		}
	}
	return nil
}

func PrintTableAlters(c *git.Client, tablename string, from, to git.TreeID) error {
	diffs, err := git.DiffTree(c, &git.DiffTreeOptions{}, from, to, nil)
	if err != nil {
		return err
	}

	for _, coldiff := range diffs {
		if coldiff.Dst.FileMode == 0 {
			fmt.Printf("ALTER TABLE %s DROP COLUMN %s;\n", tablename, coldiff.Name)
		} else if coldiff.Src.FileMode == 0 {
			col, err := GetColumn(c, ColumnName(coldiff.Name), coldiff.Dst)
			if err != nil {
				continue
			}
			fmt.Printf("ALTER TABLE %s ADD COLUMN %s;\n", tablename, col)

		} else {
			col, err := GetColumn(c, ColumnName(coldiff.Name), coldiff.Dst)
			if err != nil {
				continue
			}
			fmt.Printf("ALTER TABLE %s MODIFY COLUMN %s;\n", tablename, col)
		}
	}
	return nil
}
