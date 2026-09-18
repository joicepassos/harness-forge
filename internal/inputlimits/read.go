package inputlimits

import (
	"fmt"
	"io"
	"os"
)

// ReadFile reads no more than limit bytes and reports a safe category on overflow.
func ReadFile(path string, limit int64, category string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds the %d byte limit", category, limit)
	}
	return data, nil
}
