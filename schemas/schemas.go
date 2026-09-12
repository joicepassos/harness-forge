package schemas

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"io"
	"sync"
)

//go:embed *.schema.json
var files embed.FS
var cache sync.Map

func Read(name string) ([]byte, error) { return files.ReadFile(name) }
func Validate(name string, data []byte) error {

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	value, err := uniqueValue(dec)
	if err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	compiled, ok := cache.Load(name)
	if !ok {
		document, err := Read(name)
		if err != nil {
			return err
		}
		schema, err := jsonschema.CompileString(name, string(document))
		if err != nil {
			return err
		}
		compiled, _ = cache.LoadOrStore(name, schema)
	}
	return compiled.(*jsonschema.Schema).Validate(value)
}
