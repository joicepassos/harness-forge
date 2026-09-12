package schemas

import (
	"encoding/json"
	"fmt"
)

func uniqueValue(dec *json.Decoder) (any, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := map[string]any{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return nil, err
			}
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("expected string key")
			}
			if _, ok := object[name]; ok {
				return nil, fmt.Errorf("duplicate JSON field %q", name)
			}
			value, err := uniqueValue(dec)
			if err != nil {
				return nil, err
			}
			object[name] = value
		}
		_, err := dec.Token()
		return object, err
	case '[':
		values := []any{}
		for dec.More() {
			value, err := uniqueValue(dec)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		_, err := dec.Token()
		return values, err
	}
	return nil, fmt.Errorf("unexpected JSON delimiter")
}
