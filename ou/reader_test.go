package ou

import (
	"bytes"
	"io"
	"os"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func Test(t *testing.T) {

	// f, _ := os.Open(`E:\Spiele\Gothic1\Data\ModVDF\OU.BIN`)
	f, _ := os.Open(`C:\Users\johnh\Downloads\OU.BIN`)
	b, err := io.ReadAll(f)

	lib, err := Load(bytes.NewReader(b), charmap.Windows1251.NewDecoder())

	_ = lib
	_ = err

	//writeCsl(lib)
}
