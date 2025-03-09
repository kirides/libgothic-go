package ou

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/text/encoding"
)

const OUTimeFormat = "2.1.2006 3:4:5"

func WriteCsl(lib *lib, writer io.Writer, enc *encoding.Encoder) {
	bw := bufio.NewWriterSize(enc.Writer(writer), 4*1024*1024)
	defer bw.Flush()

	bw.WriteString("ZenGin Archive\n")
	bw.WriteString("ver 1\n")
	bw.WriteString("zCArchiverGeneric\n")
	bw.WriteString("ASCII\n")
	bw.WriteString("saveGame 0\n")
	fmt.Fprintf(bw, "date %s\n", time.Now().Format(OUTimeFormat))
	bw.WriteString("user bin2csl\n")
	bw.WriteString("END\n")
	fmt.Fprintf(bw, "objects %s\n", padRight(fmt.Sprintf("%d", lib.TotalCount()+1), 9, " "))
	bw.WriteString("END\n\n")
	bw.WriteString("[% zCCSLib 0 0]\n")
	writeLine(bw, 1, "NumOfItems=int:%d", len(lib.Blocks))

	var counter int
	for _, v := range lib.Blocks {
		writeBlock(bw, &counter, &v, 1)
	}
	bw.WriteString("[]\n")
}

func padRight(s1 string, i int, s2 string) string {
	if len(s1) < i {
		padding := strings.Repeat(s2, i-len(s1))
		return s1 + padding
	}
	return s1
}

var depthItems = []string{
	"",
	"\t",
	"\t\t",
	"\t\t\t",
	"\t\t\t\t",
	"\t\t\t\t\t",
}

func writeLine(w *bufio.Writer, depth int, format string, args ...interface{}) {
	w.WriteString(depthItems[depth])
	fmt.Fprintf(w, format, args...)
	w.WriteRune('\n')
}
func write(w *bufio.Writer, depth int, value string) {
	w.WriteString(depthItems[depth])
	w.WriteString(value)
}
func writeBlock(w *bufio.Writer, counter *int, block *block, depth int) {
	*counter++
	writeLine(w, depth, "[%% zCCSBlock 0 %d]", *counter)
	depth++

	write(w, depth, "blockName=string:")
	w.WriteString(block.BlockName)
	w.WriteRune('\n')

	write(w, depth, "numOfBlocks=int:")
	fmt.Fprintf(w, "%d", len(block.Blocks))
	w.WriteRune('\n')

	write(w, depth, "subBlock0=float:0\n")

	for _, v := range block.Blocks {
		writeAtomicBlock(w, counter, &v, depth)
	}

	depth--

	writeLine(w, depth, "[]")
}

func writeAtomicBlock(w *bufio.Writer, counter *int, block *block, depth int) {
	*counter++
	writeLine(w, depth, "[%% zCCSAtomicBlock 0 %d]", *counter)
	depth++

	for _, v := range block.Blocks {
		writeoCMsgConversationBlock(w, counter, &v, depth)
	}

	depth--

	writeLine(w, depth, "[]")
}

func writeoCMsgConversationBlock(w *bufio.Writer, counter *int, block *block, depth int) {
	*counter++
	writeLine(w, depth, "[%% oCMsgConversation:oCNpcMessage:zCEventMessage 0 %d]", *counter)
	depth++

	write(w, depth, "subType=enum:0\n")

	write(w, depth, "text=string:")
	w.WriteString(block.Text)
	w.WriteRune('\n')

	write(w, depth, "name=string:")
	w.WriteString(block.Name)
	w.WriteRune('\n')

	depth--

	writeLine(w, depth, "[]")
}
