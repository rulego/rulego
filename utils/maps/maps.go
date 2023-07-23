package maps

import "github.com/mitchellh/mapstructure"

// Map2Struct Decode takes an input structure and uses reflection to translate it to
// the output structure. output must be a pointer to a map or struct.
func Map2Struct(input interface{}, output interface{}) error {
	return mapstructure.Decode(input, output)
}
