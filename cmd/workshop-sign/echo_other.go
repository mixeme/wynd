//go:build !windows

package main

// noEcho — не на Windows пароль виден при вводе: подписывают выпуски на
// Windows (scripts\sign-workshop.bat), здесь — только для проверок.
func noEcho() func() { return func() {} }
