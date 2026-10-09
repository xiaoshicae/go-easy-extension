// Package ext (a) declares an extension point named Point; package b declares another one with the same name.
package ext

type Point interface{ Name() string }
