package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"gothic/ou"
	"maps"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

var supportedEncodings = map[string]func() encoding.Encoding{
	"1250": func() encoding.Encoding { return charmap.Windows1250 },
	"1251": func() encoding.Encoding { return charmap.Windows1251 },
	"1252": func() encoding.Encoding { return charmap.Windows1252 },
	"1253": func() encoding.Encoding { return charmap.Windows1253 },
	"1254": func() encoding.Encoding { return charmap.Windows1254 },
	"1255": func() encoding.Encoding { return charmap.Windows1255 },
	"1256": func() encoding.Encoding { return charmap.Windows1256 },
	"1257": func() encoding.Encoding { return charmap.Windows1257 },
	"1258": func() encoding.Encoding { return charmap.Windows1258 },
}

func main() {
	encodingsText := strings.Builder{}
	encodingsSlice := make([]string, 0)

	for v := range maps.Keys(supportedEncodings) {
		encodingsSlice = append(encodingsSlice, v)
	}
	slices.Sort(encodingsSlice)

	for _, v := range encodingsSlice {
		encodingsText.WriteString(v)
		switch v := supportedEncodings[v]().(type) {
		case *charmap.Charmap:
			encodingsText.WriteString(" - ")
			encodingsText.WriteString(v.String())
		}
		encodingsText.WriteString("\n")
	}

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Usage:
bin2csl.exe ENCODING SOURCE_OU_BIN_FILE DESINATION_OU_CSL_FILE

Examples:

-- Convert a ou.bin to it's csl version with the correct encoding
bin2csl.exe 1251 "C:\path with spaces needs quotes\OU.BIN" c:\temp\ou.csl

Available encodings:
%s`, encodingsText.String())
	}

	flag.Parse()

	args := flag.Args()

	if len(args) != 3 {
		flag.Usage()
		os.Exit(1)
		return
	}

	appCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	_ = appCtx

	enc := supportedEncodings[args[0]]()
	srcPath := args[1]
	dstPath := args[2]
	data, err := os.ReadFile(srcPath)
	if err != nil {
		panic(err)
	}
	lib, err := ou.Load(bytes.NewReader(data), enc.NewDecoder())
	if err != nil {
		panic(err)
	}
	buf := bytes.NewBuffer(nil)
	buf.Grow(len(data) * 2)

	ou.WriteCsl(lib, buf, enc.NewEncoder())
	os.WriteFile(dstPath, buf.Bytes(), 0o777)
}
