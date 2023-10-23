package fieldmapper

import (
	"reflect"
	"strings"

	"github.com/dop251/goja/parser"
)

const tagName string = "lean"

type FallbackFieldMapper struct {
}

func (ffm FallbackFieldMapper) FieldName(_ reflect.Type, f reflect.StructField) string {
	tag := f.Tag.Get(tagName)
	if idx := strings.IndexByte(tag, ','); idx != -1 {
		tag = tag[:idx]
	}
	if parser.IsIdentifier(tag) {
		return tag
	}
	return f.Name
}

func (ffm FallbackFieldMapper) MethodName(_ reflect.Type, m reflect.Method) string {
	return m.Name
}
