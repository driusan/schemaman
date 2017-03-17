package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/driusan/dgit/git"
)

type ColumnName string
type Engine string
type CharEncoding string
type DBEngine string

type TableMetadata struct {
	PrimaryKey ColumnName
	Order      []ColumnName
	CharSet    CharEncoding
	Engine     DBEngine
}

func ParseMetaData(c *git.Client, metacolumn git.TreeEntry) TableMetadata {
	var val TableMetadata
	mVal, err := git.CatFile(c, metacolumn.Sha1, git.CatFileOptions{Pretty: true})
	if err != nil {
		return val
	}

	lines := strings.Split(mVal, "\n")
	for _, line := range lines {
		switch l := strings.TrimSpace(line); {
		case strings.HasPrefix(l, "Primary Key:"):
			data := strings.TrimPrefix(l, "Primary Key:")
			val.PrimaryKey = ColumnName(strings.TrimSpace(data))
		case strings.HasPrefix(l, "Column Order:"):
			data := strings.TrimPrefix(l, "Column Order:")
			cols := strings.Split(data, ",")
			for _, c := range cols {
				val.Order = append(val.Order, ColumnName(strings.TrimSpace(c)))
			}
		case strings.HasPrefix(l, "Character Set:"):
			data := strings.TrimPrefix(l, "Character Set:")
			val.CharSet = CharEncoding(strings.TrimSpace(data))
		case strings.HasPrefix(l, "Engine:"):
			data := strings.TrimPrefix(l, "Engine:")
			val.Engine = DBEngine(strings.TrimSpace(data))

		}
	}

	return val
}

func PrintTable(c *git.Client, name string, treebase git.TreeEntry) error {
	if treebase.FileMode != git.ModeTree {
		return fmt.Errorf("Invalid table: %s", name)
	}

	var metadata TableMetadata
	tree := git.TreeID(treebase.Sha1)

	columns, err := tree.GetAllObjects(c, "", false, false)
	if err != nil {
		return err
	}

	if md, ok := columns[".metadata"]; ok {
		metadata = ParseMetaData(c, md)
	}

	// Make sure the metadata.Order has all columns.
	if metadata.Order == nil {
		metadata.Order = make([]ColumnName, 0, len(columns))
		for name := range columns {
			if name != "" && name[0] != '.' {
				metadata.Order = append(metadata.Order, ColumnName(name))
			}
		}
	} else {
	colLoop:
		// Add any missing columns, if order didn't have them but
		// they're on the table.
		for name := range columns {
			for _, cname := range metadata.Order {
				if cname == ColumnName(name) {
					continue colLoop
				}
			}
			metadata.Order = append(metadata.Order, ColumnName(name))
		}
	}

	var cols []string
	for _, cname := range metadata.Order {
		if cname == "" || cname[0] == '.' {
			continue
		}
		if column, ok := columns[git.IndexPath(cname)]; ok {
			col, err := GetColumn(c, cname, column)
			if err != nil {
				continue
			}
			cols = append(cols, col)
		}

	}
	if len(cols) > 0 {
		fmt.Printf("\nCREATE TABLE %s (\n", name)
		fmt.Printf("\t%s", strings.Join(cols, ",\n\t"))
		fmt.Print("\n)")
	}
	if metadata.CharSet != "" {
		fmt.Printf(" CHARACTER SET = %s", metadata.CharSet)
	}
	if metadata.Engine != "" {
		fmt.Printf(" ENGINE = %s", metadata.Engine)
	}
	fmt.Println(";")
	return nil
}

func GetColumn(c *git.Client, name ColumnName, treebase git.TreeEntry) (string, error) {
	var col string
	tree := git.TreeID(treebase.Sha1)

	coldata, err := tree.GetAllObjects(c, "", false, false)
	if err != nil {
		return "", err
	}
	typ, ok := coldata["type"]
	if !ok {
		// It's not something we care about, but not necessarily an error
		return "", fmt.Errorf("Invalid column")
	}
	tVal, err := git.CatFile(c, typ.Sha1, git.CatFileOptions{Pretty: true})
	if err != nil {
		return "", err
	}
	tVal = strings.TrimSpace(tVal)
	col = fmt.Sprintf("%s %s", name, tVal)
	if _, ok := coldata["not_null"]; ok {
		col += " NOT NULL"
	}
	if defval, ok := coldata["default"]; ok {
		dVal, err := git.CatFile(c, defval.Sha1, git.CatFileOptions{Pretty: true})
		if err != nil {
			return "", err
		}
		dVal = strings.TrimSpace(dVal)
		// FIXME: There should be a better way to determine if the
		// default needs quotes, this isn't very reliable.. but the
		// information_schema when importing doesn't add quotes around
		// strings that need quotes.
		// For now, just treat 'CURRENT_TIMESTAMP' and integers specially.
		// (The way of determining if the column is an integer also isn't
		// very reliable.)
		if dVal == "CURRENT_TIMESTAMP" || strings.Contains(tVal, "int") {
			col += fmt.Sprintf(" DEFAULT %s", dVal)
		} else {
			col += fmt.Sprintf(" DEFAULT '%s'", dVal)
		}
	}

	if _, ok := coldata["auto_increment"]; ok {
		col += " AUTO_INCREMENT"
	}
	return col, nil
}

func ShowTables(c *git.Client, tablesTree git.Sha1, tableList []string) error {
	tree := git.TreeID(tablesTree)
	objs, err := tree.GetAllObjects(c, "", false, false)
	if err != nil {
		return err
	}

	for name, i := range objs {
		if len(tableList) > 0 {
			for _, val := range tableList {
				if val == name.String() {
					goto showThisTable
				}
			}
			continue
		}
	showThisTable:
		if err := PrintTable(c, name.String(), i); err != nil {
			log.Println(err)
		}
	}
	return nil
}

func Show(c *git.Client, args []string) error {
	var rev string
	if len(args) == 0 {
		rev = "HEAD:.schema/tables"
	} else if strings.Contains(args[0], ":") {
		rev = args[0]
	} else {
		rev = args[0] + ":.schema/tables"
	}
	/*
		This doesn't work, because RevparseTreeish isn't smart enough to handle
		pathspecs yet, so we just call out to the real git client instead of
		doing it in pure Go (and die a little inside.)
		_, err = git.RevParseTreeish(c, nil, "HEAD:.schema/tables")
		if err != nil {
			log.Fatal(err)
		}
	*/
	cmd := exec.Command("git", "rev-parse", rev)
	val, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	if len(val) != 41 {
		// 40 bytes for the hash as a hexadecimal string, 1 for the
		// newline
		log.Fatalf("No valid table schema found.")
	}
	sha, err := git.Sha1FromString(string(val[:40]))
	if err != nil {
		log.Fatal(err)
	}

	if len(args) >= 1 {
		args = args[1:]
	}
	if err := ShowTables(c, sha, args); err != nil {
		log.Fatalln(err)
	}
	return nil
}
