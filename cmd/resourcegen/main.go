package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"

	"dualcenter/internal/version"
)

const (
	coffAMD64                 = 0x8664
	relocAMD64Addr32NB        = 0x0003
	resourceSectionAttributes = 0x40300040
)

type iconImage struct {
	width, height, colors, reserved byte
	planes, bits                    uint16
	data                            []byte
	id                              uint16
}

type resourceItem struct {
	id              uint32
	data            []byte
	languageOffset  int
	dataEntryOffset int
	dataOffset      int
}

type resourceType struct {
	id     uint32
	offset int
	items  []*resourceItem
}

func main() {
	iconPath := flag.String("icon", "assets/DualCenter.ico", "arquivo ICO")
	manifestPath := flag.String("manifest", "app.manifest", "manifesto XML")
	outPath := flag.String("out", "resource_amd64.syso", "arquivo COFF de saída")
	description := flag.String("description", "DualCenter", "descrição do arquivo")
	filename := flag.String("filename", "DualCenter.exe", "nome original do arquivo")
	flag.Parse()

	icons, err := parseICO(*iconPath)
	check(err)
	manifest, err := os.ReadFile(*manifestPath)
	check(err)
	manifestText := string(manifest)
	versionedManifest := strings.ReplaceAll(manifestText, `version="0.0.0.0"`, `version="`+version.FileVersion()+`"`)
	if versionedManifest == manifestText {
		check(fmt.Errorf("o manifesto %s não contém o marcador de versão 0.0.0.0", *manifestPath))
	}
	manifest = []byte(versionedManifest)
	versionBlob, err := buildVersionInfo(*description, *filename)
	check(err)

	iconItems := make([]*resourceItem, 0, len(icons))
	for _, image := range icons {
		iconItems = append(iconItems, &resourceItem{id: uint32(image.id), data: image.data})
	}
	types := []*resourceType{
		{id: 3, items: iconItems},
		{id: 14, items: []*resourceItem{{id: 1, data: buildGroupIcon(icons)}}},
		{id: 16, items: []*resourceItem{{id: 1, data: versionBlob}}},
		{id: 24, items: []*resourceItem{{id: 1, data: manifest}}},
	}

	rsrc, relocations := buildResourceSection(types)
	check(writeCOFF(*outPath, rsrc, relocations))
	fmt.Printf("%s: %s (%s)\n", *outPath, version.Display(), version.FileVersion())
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "resourcegen:", err)
		os.Exit(1)
	}
}

func parseICO(path string) ([]iconImage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 6 {
		return nil, fmt.Errorf("ICO truncado")
	}
	reserved := binary.LittleEndian.Uint16(data[0:2])
	kind := binary.LittleEndian.Uint16(data[2:4])
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if reserved != 0 || kind != 1 || count < 1 || len(data) < 6+count*16 {
		return nil, fmt.Errorf("cabeçalho ICO inválido")
	}
	images := make([]iconImage, 0, count)
	for i := 0; i < count; i++ {
		pos := 6 + i*16
		size := int(binary.LittleEndian.Uint32(data[pos+8 : pos+12]))
		offset := int(binary.LittleEndian.Uint32(data[pos+12 : pos+16]))
		if size <= 0 || offset < 0 || offset+size > len(data) {
			return nil, fmt.Errorf("imagem %d do ICO truncada", i+1)
		}
		images = append(images, iconImage{
			width:    data[pos],
			height:   data[pos+1],
			colors:   data[pos+2],
			reserved: data[pos+3],
			planes:   binary.LittleEndian.Uint16(data[pos+4 : pos+6]),
			bits:     binary.LittleEndian.Uint16(data[pos+6 : pos+8]),
			data:     append([]byte(nil), data[offset:offset+size]...),
			id:       uint16(i + 1),
		})
	}
	return images, nil
}

func buildGroupIcon(images []iconImage) []byte {
	out := make([]byte, 6+len(images)*14)
	binary.LittleEndian.PutUint16(out[2:4], 1)
	binary.LittleEndian.PutUint16(out[4:6], uint16(len(images)))
	for i, image := range images {
		pos := 6 + i*14
		out[pos] = image.width
		out[pos+1] = image.height
		out[pos+2] = image.colors
		out[pos+3] = image.reserved
		binary.LittleEndian.PutUint16(out[pos+4:pos+6], image.planes)
		binary.LittleEndian.PutUint16(out[pos+6:pos+8], image.bits)
		binary.LittleEndian.PutUint32(out[pos+8:pos+12], uint32(len(image.data)))
		binary.LittleEndian.PutUint16(out[pos+12:pos+14], image.id)
	}
	return out
}

func utf16z(text string) []byte {
	units := utf16.Encode([]rune(text + "\x00"))
	out := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], unit)
	}
	return out
}

func align4(data []byte) []byte {
	for len(data)%4 != 0 {
		data = append(data, 0)
	}
	return data
}

func versionNode(key string, value []byte, valueLength uint16, valueType uint16, children ...[]byte) []byte {
	out := make([]byte, 6)
	out = append(out, utf16z(key)...)
	out = align4(out)
	out = append(out, value...)
	out = align4(out)
	for _, child := range children {
		out = append(out, child...)
		out = align4(out)
	}
	binary.LittleEndian.PutUint16(out[0:2], uint16(len(out)))
	binary.LittleEndian.PutUint16(out[2:4], valueLength)
	binary.LittleEndian.PutUint16(out[4:6], valueType)
	return out
}

func buildVersionInfo(description, filename string) ([]byte, error) {
	parts := strings.Split(version.FileVersion(), ".")
	if len(parts) != 4 {
		return nil, fmt.Errorf("versão de arquivo inválida: %s", version.FileVersion())
	}
	numbers := make([]uint32, 4)
	for i, part := range parts {
		value, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("versão inválida: %w", err)
		}
		numbers[i] = uint32(value)
	}
	ms := numbers[0]<<16 | numbers[1]
	ls := numbers[2]<<16 | numbers[3]
	fixedValues := []uint32{
		0xFEEF04BD, 0x00010000,
		ms, ls, ms, ls,
		0x0000003F, 0,
		0x00040004, 1, 0, 0, 0,
	}
	var fixed bytes.Buffer
	for _, value := range fixedValues {
		_ = binary.Write(&fixed, binary.LittleEndian, value)
	}
	display := version.FileVersion()
	if version.Current.Label != "" {
		display += " " + version.Current.Label
	}
	values := [][2]string{
		{"CompanyName", version.Current.Publisher},
		{"FileDescription", description},
		{"FileVersion", display},
		{"InternalName", "DualCenter"},
		{"OriginalFilename", filename},
		{"ProductName", "DualCenter"},
		{"ProductVersion", display},
		{"LegalCopyright", "Copyright © 2026 " + version.Current.Publisher},
	}
	stringChildren := make([][]byte, 0, len(values))
	for _, pair := range values {
		encoded := utf16z(pair[1])
		stringChildren = append(stringChildren, versionNode(pair[0], encoded, uint16(len([]rune(pair[1]))+1), 1))
	}
	stringTable := versionNode("041604B0", nil, 0, 1, stringChildren...)
	stringInfo := versionNode("StringFileInfo", nil, 0, 1, stringTable)
	translation := make([]byte, 4)
	binary.LittleEndian.PutUint16(translation[0:2], 0x0416)
	binary.LittleEndian.PutUint16(translation[2:4], 1200)
	varInfo := versionNode("VarFileInfo", nil, 0, 1, versionNode("Translation", translation, 4, 0))
	return versionNode("VS_VERSION_INFO", fixed.Bytes(), uint16(fixed.Len()), 0, stringInfo, varInfo), nil
}

func buildResourceSection(types []*resourceType) ([]byte, []uint32) {
	data := make([]byte, 16+len(types)*8)
	alloc := func(size int) int {
		offset := len(data)
		data = append(data, make([]byte, size)...)
		return offset
	}
	for _, typ := range types {
		typ.offset = alloc(16 + len(typ.items)*8)
		for _, item := range typ.items {
			item.languageOffset = alloc(24)
		}
	}
	data = align4(data)
	for _, typ := range types {
		for _, item := range typ.items {
			item.dataEntryOffset = alloc(16)
		}
	}
	data = align4(data)
	for _, typ := range types {
		for _, item := range typ.items {
			data = align4(data)
			item.dataOffset = len(data)
			data = append(data, item.data...)
		}
	}

	binary.LittleEndian.PutUint16(data[14:16], uint16(len(types)))
	var relocations []uint32
	for typeIndex, typ := range types {
		rootEntry := 16 + typeIndex*8
		binary.LittleEndian.PutUint32(data[rootEntry:rootEntry+4], typ.id)
		binary.LittleEndian.PutUint32(data[rootEntry+4:rootEntry+8], uint32(typ.offset)|0x80000000)
		binary.LittleEndian.PutUint16(data[typ.offset+14:typ.offset+16], uint16(len(typ.items)))
		for itemIndex, item := range typ.items {
			itemEntry := typ.offset + 16 + itemIndex*8
			binary.LittleEndian.PutUint32(data[itemEntry:itemEntry+4], item.id)
			binary.LittleEndian.PutUint32(data[itemEntry+4:itemEntry+8], uint32(item.languageOffset)|0x80000000)
			binary.LittleEndian.PutUint16(data[item.languageOffset+14:item.languageOffset+16], 1)
			binary.LittleEndian.PutUint32(data[item.languageOffset+16:item.languageOffset+20], 0x0416)
			binary.LittleEndian.PutUint32(data[item.languageOffset+20:item.languageOffset+24], uint32(item.dataEntryOffset))
			binary.LittleEndian.PutUint32(data[item.dataEntryOffset:item.dataEntryOffset+4], uint32(item.dataOffset))
			binary.LittleEndian.PutUint32(data[item.dataEntryOffset+4:item.dataEntryOffset+8], uint32(len(item.data)))
			relocations = append(relocations, uint32(item.dataEntryOffset))
		}
	}
	return data, relocations
}

func writeCOFF(path string, rsrc []byte, relocations []uint32) error {
	const fileHeaderSize = 20
	const sectionHeaderSize = 40
	rawOffset := uint32(fileHeaderSize + sectionHeaderSize)
	relocationOffset := rawOffset + uint32(len(rsrc))
	symbolOffset := relocationOffset + uint32(len(relocations))*10

	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(coffAMD64))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, symbolOffset)
	_ = binary.Write(&out, binary.LittleEndian, uint32(2))
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))

	name := [8]byte{'.', 'r', 's', 'r', 'c'}
	_ = binary.Write(&out, binary.LittleEndian, name)
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(rsrc)))
	_ = binary.Write(&out, binary.LittleEndian, rawOffset)
	_ = binary.Write(&out, binary.LittleEndian, relocationOffset)
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint16(len(relocations)))
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(resourceSectionAttributes))
	out.Write(rsrc)
	for _, offset := range relocations {
		_ = binary.Write(&out, binary.LittleEndian, offset)
		_ = binary.Write(&out, binary.LittleEndian, uint32(0))
		_ = binary.Write(&out, binary.LittleEndian, uint16(relocAMD64Addr32NB))
	}

	symbol := make([]byte, 18)
	copy(symbol[0:8], []byte(".rsrc"))
	binary.LittleEndian.PutUint16(symbol[12:14], 1)
	symbol[16] = 3
	symbol[17] = 1
	out.Write(symbol)
	aux := make([]byte, 18)
	binary.LittleEndian.PutUint32(aux[0:4], uint32(len(rsrc)))
	binary.LittleEndian.PutUint16(aux[4:6], uint16(len(relocations)))
	out.Write(aux)
	_ = binary.Write(&out, binary.LittleEndian, uint32(4))

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}
