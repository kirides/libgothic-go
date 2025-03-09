package ou

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

type ouReader struct {
	decoder    *encoding.Decoder
	win1252Dec *encoding.Decoder
	fieldNames []string
}

func Load(rdr io.ReadSeeker, decoder *encoding.Decoder) (*lib, error) {
	result := &lib{}

	if err := findBegin(rdr); err != nil {
		return nil, err
	}
	buffer := make([]byte, 4096)

	binVersion, err := readInt(rdr, buffer)
	if err != nil {
		return nil, err
	}
	if binVersion != 2 && binVersion != 1 {
		return nil, errors.New("unsupported OU version")
	}

	numOU, err := readInt(rdr, buffer)
	if err != nil {
		return nil, err
	}

	chunk_pos, err := readInt(rdr, buffer)
	if err != nil {
		return nil, err
	}
	here, err := rdr.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("could not save position. %w", err)
	}

	if _, err := rdr.Seek(int64(chunk_pos), io.SeekStart); err != nil {
		return nil, fmt.Errorf("could not move to fieldnames. %w", err)
	}

	fields, err := getFieldNames(rdr, buffer)
	if err != nil {
		return nil, fmt.Errorf("could not get fieldnames. %w", err)
	}

	if _, err := rdr.Seek(here, io.SeekStart); err != nil {
		return nil, fmt.Errorf("could not restore position. %w", err)
	}
	cpWin1252 := charmap.Windows1252
	win1252Dec := cpWin1252.NewDecoder()

	ouReader := &ouReader{
		decoder:    decoder,
		win1252Dec: win1252Dec,
		fieldNames: fields,
	}

	result.Count = numOU

	libBlock, err := ouReader.readLib(rdr, buffer)
	if err != nil {
		return nil, fmt.Errorf("unable to parse library. %w", err)
	}
	result.Blocks = libBlock.Blocks
	return result, nil
}

func getFieldNames(rdr io.Reader, buffer []byte) ([]string, error) {
	numFields, err := readInt(rdr, buffer)
	if err != nil {
		return nil, err
	}
	fields := make([]string, numFields)

	for i := 0; i < numFields; i++ {
		strLen, err := readInt16(rdr, buffer)
		if err != nil {
			return nil, err
		}
		idx, err := readInt16(rdr, buffer)
		if err != nil {
			return nil, err
		}

		// skip hash
		if err := consumeBytes(rdr, buffer, 4); err != nil {
			return nil, err
		}

		if err := consumeBytes(rdr, buffer, int(strLen)); err != nil {
			return nil, err
		}
		fields[idx] = string(append([]byte{}, buffer[:strLen]...))
	}

	return fields, nil
}

type field struct {
	Name  string
	Value interface{}
}

func (f *field) String() string {
	return fmt.Sprintf("%v", f.Value)
}

type block struct {
	ID        string
	Fields    []*field // unused
	BlockName string
	Text      string
	Name      string
	Blocks    []block
}

func (l *block) TotalCount() int32 {
	var counter int32
	stack := make([]block, 0)
	stack = append(stack, l.Blocks...)
	for len(stack) > 0 {
		counter++
		last := len(stack) - 1
		item := stack[last]
		stack = slices.Delete(stack, last, 1)
		stack = append(stack, item.Blocks...)
	}

	return counter
}

func (b *block) Get(f string) *field {
	for i := 0; i < len(b.Fields); i++ {
		if b.Fields[i].Name == f {
			return b.Fields[i]
		}
	}
	return nil
}
func (b *block) String() string {
	sb := strings.Builder{}
	for _, v := range b.Fields {
		sb.WriteString("\t")
		sb.WriteString(v.String())
		sb.WriteString("\n")
	}
	for _, v := range b.Blocks {
		sb.WriteString("\t")
		sb.WriteString(v.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

type lib struct {
	Count  int
	Blocks []block
}

func (l *lib) TotalCount() int32 {
	var counter int32
	stack := make([]block, 0)
	stack = append(stack, l.Blocks...)
	for len(stack) > 0 {
		counter++
		last := len(stack) - 1
		item := stack[last]
		stack = slices.Delete(stack, last, last+1)
		stack = append(stack, item.Blocks...)
	}

	return counter
}

const (
	TypeString = 1
	TypeInt    = 2
	TypeFloat  = 3
	TypeByte   = 4
	TypeEnum   = 0x11
	TypeField  = 0x12
)

func (r *ouReader) readLib(rdr io.Reader, buffer []byte) (block, error) {
	if _, err := io.ReadFull(rdr, buffer[:1]); err != nil {
		return block{}, err
	}
	blockStart, err := r.parse(buffer[0], rdr, buffer)
	if err != nil {
		return block{}, err
	}

	return r.readBlock(blockStart.(string), rdr, buffer)
}

func (r *ouReader) readBlock(name string, rdr io.Reader, buffer []byte) (block, error) {
	bName := name[len("[% "):]

	idx2ndSpace := strings.Index(bName, " ")
	if idx2ndSpace == -1 {
		return block{}, fmt.Errorf("unable to identify block")
	}

	b := block{ID: bName[:idx2ndSpace]}

outerLoop:
	for {
		if _, err := io.ReadFull(rdr, buffer[:1]); err != nil {
			return block{}, err
		}

		switch buffer[0] {
		case 0:
			break outerLoop
		case TypeString:
			v, err := readString(rdr, buffer, r.decoder)
			if err != nil {
				return block{}, err
			}
			if v == "[]" {
				break outerLoop
			}
			if strings.HasPrefix(v, "[% ") {
				in, err := r.readBlock(v, rdr, buffer)
				if err != nil {
					return block{}, err
				}
				b.Blocks = append(b.Blocks, in)
			}
		case TypeField:
			fieldIdx, err := readInt(rdr, buffer)
			if err != nil {
				return block{}, err
			}
			fieldName := r.fieldNames[fieldIdx]

			if err := consumeBytes(rdr, buffer, 1); err != nil {
				return block{}, err
			}
			v, err := r.parse(buffer[0], rdr, buffer)
			if err != nil {
				return block{}, err
			}
			if fieldName == "blockName" {
				b.BlockName = v.(string)
			} else if fieldName == "text" {
				b.Text = v.(string)
			} else if fieldName == "name" {
				b.Name = v.(string)
			}
			b.Fields = append(b.Fields, &field{Name: fieldName, Value: v})
		default:
			return block{}, errors.New("unable to parse type")

		}
	}
	return b, nil
}

func (r *ouReader) parse(t byte, rdr io.Reader, buffer []byte) (interface{}, error) {
	switch t {
	case TypeString:
		return readString(rdr, buffer, r.decoder)
	case TypeByte:
		if err := consumeBytes(rdr, buffer, 1); err != nil {
			return nil, err
		}
		return byte(0), nil
	case TypeInt, TypeField, TypeEnum:
		if err := consumeBytes(rdr, buffer, 4); err != nil {
			return nil, err
		}
		return int(binary.LittleEndian.Uint32(buffer[:4])), nil
	case TypeFloat:
		if err := consumeBytes(rdr, buffer, 4); err != nil {
			return nil, err
		}
		return math.Float32frombits(binary.LittleEndian.Uint32(buffer[:4])), nil
	}

	return nil, fmt.Errorf("unable to parse type %d", buffer[0])
}

func readString(rdr io.Reader, buffer []byte, dec *encoding.Decoder) (string, error) {
	idLen, err := readInt16(rdr, buffer)
	if err != nil {
		return "", err
	}

	// Read ID
	if _, err := io.ReadFull(rdr, buffer[:idLen]); err != nil {
		return "", err
	}

	str, err := dec.String(string(buffer[:idLen]))
	if err != nil {
		return "", err
	}
	return str, nil
}

func consumeBytes(rdr io.Reader, buffer []byte, toSkip int) error {
	if _, err := io.ReadFull(rdr, buffer[:toSkip]); err != nil {
		return err
	}
	return nil
}

func readInt16skipBytes(rdr io.Reader, buffer []byte, additonal int) error {
	toSkip, err := readInt16(rdr, buffer[:2])
	if err != nil {
		return err
	}
	if _, err := io.ReadFull(rdr, buffer[:int(toSkip)+additonal]); err != nil {
		return err
	}
	return nil
}

func readInt(rdr io.Reader, buffer []byte) (int, error) {
	if _, err := io.ReadFull(rdr, buffer[:4]); err != nil {
		return 0, err
	}
	return int(binary.LittleEndian.Uint32(buffer[:4])), nil
}

func readInt16(rdr io.Reader, buffer []byte) (int16, error) {
	if _, err := io.ReadFull(rdr, buffer[:2]); err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buffer[:2])), nil
}

func findBegin(rdr io.Reader) error {
	endBuf := make([]byte, 1)
	for {
		if _, err := io.ReadFull(rdr, endBuf); err != nil {
			return err
		}
		if endBuf[0] != 0x0A {
			continue
		}
		if _, err := io.ReadFull(rdr, endBuf); err != nil {
			return err
		}
		if endBuf[0] != 'E' {
			continue
		}
		if _, err := io.ReadFull(rdr, endBuf); err != nil {
			return err
		}
		if endBuf[0] != 'N' {
			continue
		}
		if _, err := io.ReadFull(rdr, endBuf); err != nil {
			return err
		}
		if endBuf[0] == 'D' {
			break
		}
	}
	if _, err := io.ReadFull(rdr, endBuf); err != nil {
		return err
	}
	if endBuf[0] != 0x0A {
		return fmt.Errorf("expected 'END' to end with 0x0A but found %x", endBuf[0])
	}
	return nil
}
