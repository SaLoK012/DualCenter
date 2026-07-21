package version

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Info descreve a única versão usada pelo aplicativo, instalador e build.
type Info struct {
	Version   string `json:"version"`
	Build     int    `json:"build"`
	Label     string `json:"label"`
	Publisher string `json:"publisher"`
}

//go:embed version.json
var raw []byte

// Current é carregada do mesmo arquivo consumido pelo gerador de recursos.
var Current = mustParse(raw)

func mustParse(data []byte) Info {
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		panic(fmt.Sprintf("versão do DualCenter inválida: %v", err))
	}
	if info.Version == "" || info.Build <= 0 || info.Publisher == "" {
		panic("versão do DualCenter incompleta")
	}
	if _, err := numericVersionParts(info.Version); err != nil {
		panic(fmt.Sprintf("versão pública do DualCenter inválida: %v", err))
	}
	return info
}

func numericVersionParts(value string) ([3]uint32, error) {
	var numbers [3]uint32
	parts := strings.Split(value, ".")
	if len(parts) != 2 && len(parts) != 3 {
		return numbers, fmt.Errorf("%q deve ter dois ou três componentes", value)
	}
	for i, part := range parts {
		n, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return numbers, fmt.Errorf("componente %q inválido", part)
		}
		numbers[i] = uint32(n)
	}
	return numbers, nil
}

// Display retorna a versão apresentada na interface.
func Display() string {
	if Current.Label == "" {
		return fmt.Sprintf("v%s (Build %d)", Current.Version, Current.Build)
	}
	return fmt.Sprintf("v%s %s (Build %d)", Current.Version, Current.Label, Current.Build)
}

// FileVersion retorna os quatro componentes numéricos usados pelo Windows.
func FileVersion() string {
	parts, err := numericVersionParts(Current.Version)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%d.%d.%d.%d", parts[0], parts[1], parts[2], Current.Build)
}
