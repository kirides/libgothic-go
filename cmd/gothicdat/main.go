package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"gothic/dat"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

func main() {
	outVar := flag.String("o", "-", "output file")
	verboseVar := flag.Bool("v", false, "verbose output, full symbol signature")
	flag.Parse()
	args := flag.Args()
	// args = []string{
	// 	`GOTHIC.DAT`,
	// }
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "ERR: no file provided\n")
		os.Exit(1)
	}
	buf, err := os.ReadFile(args[0])
	if err != nil {
		panic(err)
	}

	var out io.Writer = os.Stdout
	if *outVar != "-" {
		f, err := os.Create(*outVar)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		w := bufio.NewWriter(f)
		defer w.Flush()
		out = w
	}
	filterFn := func(v *dat.Symbol) bool {
		// if v.Type != loader.TypeFunc && v.Type != loader.TypeInstance {
		// 	return false
		// }

		return !strings.ContainsRune(v.Name, '.')
	}
	l := dat.NewLoader(charmap.Windows1252)
	if len(args) > 1 {
		indices := map[int]struct{}{}
		for i := 1; i < len(args); i++ {
			v, err := strconv.ParseInt(args[i], 16, 32)
			if err != nil {
				panic(err)
			}
			indices[int(v)] = struct{}{}
		}

		filterFn = func(v *dat.Symbol) bool {
			if _, ok := indices[v.Index]; ok {
				return true
			}
			return false
		}
		l.BailOutIndex = -1
		for k := range indices {
			if k > l.BailOutIndex {
				l.BailOutIndex = k + 1
			}
		}
	}
	symbols, _ := l.LoadFilter(bytes.NewReader(buf), filterFn)
	if *verboseVar {
		for _, v := range symbols {
			fmt.Fprintf(out, "%10d|%08x|%s\n", v.Index, v.Index, v)
		}
	} else {
		for _, v := range symbols {
			fmt.Fprintf(out, "%10d|%08x|%s\n", v.Index, v.Index, v.Name)
		}
	}
}
