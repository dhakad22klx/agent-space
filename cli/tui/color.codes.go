package tui

const reset = "\x1b[0m"

func Red(s string) string     { return "\x1b[31m" + s + reset }
func Green(s string) string   { return "\x1b[32m" + s + reset }
func Yellow(s string) string  { return "\x1b[33m" + s + reset }
func Blue(s string) string    { return "\x1b[34m" + s + reset }
func Magenta(s string) string { return "\x1b[35m" + s + reset }
func Cyan(s string) string    { return "\x1b[36m" + s + reset }
func White(s string) string   { return "\x1b[37m" + s + reset }
func Gray(s string) string    { return "\x1b[90m" + s + reset }
func Bold(s string) string    { return "\x1b[1m" + s + reset }
func Orange(s string) string  { return "\x1b[38;5;208m" + s + reset } // 256-color palette

//https://gist.github.com/fnky/458719343aabd01cfb17a3a4f7296797
