package gojautils

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/dop251/goja"
)

var allCapsMatcher = regexp.MustCompile(`^[[:upper:]]+$`)

func smartUncapitalize(s string) string {

	if allCapsMatcher.MatchString(s) {
		return strings.ToLower(s)
	}

	return strings.ToLower(s[0:1]) + s[1:]

}

type smartCapFieldNameMapper struct{}

func (u smartCapFieldNameMapper) FieldName(_ reflect.Type, f reflect.StructField) string {
	return smartUncapitalize(f.Name)
}

func (u smartCapFieldNameMapper) MethodName(_ reflect.Type, m reflect.Method) string {
	return smartUncapitalize(m.Name)
}

var SmartCapFieldNameMapper goja.FieldNameMapper = smartCapFieldNameMapper{}
