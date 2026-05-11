package main

// commandOrder defines the canonical display order for --help --all.
var commandOrder = []string{
	"validate", "check", "ready", "next-id", "new",
	"close", "log", "archive", "migrate", "init", "version", "update",
}

// helpText maps each command name to its per-file help const. Each entry's
// value is defined as `const help<Cmd>` in the matching command file (the
// authoritative source of truth for that command's user-facing summary).
// Every entry begins with a `## erg COMMAND ARGS` header line, required by
// `erg --help --all` and the generated manual.
var helpText = map[string]string{
	"validate": helpValidate,
	"check":    helpCheck,
	"ready":    helpReady,
	"next-id":  helpNextID,
	"new":      helpNew,
	"close":    helpClose,
	"log":      helpLog,
	"archive":  helpArchive,
	"migrate":  helpMigrate,
	"init":     helpInit,
	"version":  helpVersion,
	"update":   helpUpdate,
}
