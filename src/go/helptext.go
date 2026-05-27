package main

// commandEntry pairs a command name with its usage args, one-line
// summary, and full help text. Adding or reordering a command is a
// single-line change in the commands slice below.
type commandEntry struct {
	Name    string // command name (e.g. "validate")
	Args    string // argument synopsis (e.g. "FILES...")
	Summary string // one-liner for printUsage (from summary<Cmd> const)
	Help    string // full help text for --help (from help<Cmd> const)
}

// commands is the single ordered registry of all erg subcommands.
// Display order matches --help and --help --all output.
var commands = []commandEntry{
	{"validate", "FILES...", summaryValidate, helpValidate},
	{"check", "[DIR]", summaryCheck, helpCheck},
	{"list", "[DIR] [--all] [--json]", summaryList, helpList},
	{"ready", "[DIR] [--json]", summaryReady, helpReady},
	{"next-id", "[DIR]", summaryNextID, helpNextID},
	{"new", "TITLE [DIR]", summaryNew, helpNew},
	{"close", "ID|FILE REASON [DIR]", summaryClose, helpClose},
	{"log", "ID LINE [DIR]", summaryLog, helpLog},
	{"tag", "ID TAGNAME [DIR]", summaryTag, helpTag},
	{"untag", "ID TAGNAME [DIR]", summaryUntag, helpUntag},
	{"archive", "[ID...] [DIR]", summaryArchive, helpArchive},
	{"migrate", "[DIR]", summaryMigrate, helpMigrate},
	{"init", "[DIR]", summaryInit, helpInit},
	{"version", "", summaryVersion, helpVersion},
	{"update", "", summaryUpdate, helpUpdate},
}

// commandAliases maps an alternate name to its canonical command. Resolved
// once in main before help lookup and dispatch, so an alias behaves exactly
// like its canonical name everywhere (e.g. `erg ls` == `erg list`).
var commandAliases = map[string]string{
	"ls": "list",
}
