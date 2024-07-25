package sniper

import (
	"fmt"
	"os"
)

// ParseFile parses a file
func ParseFile(lang Language, filePath string) (ParsedFile, error) {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not parse file: %v", err)
	}

	switch lang {
	case LangPy:
		return ParsePython(filePath, source)
	default:
		return nil, fmt.Errorf("language not supported: %v", lang)
	}
}
