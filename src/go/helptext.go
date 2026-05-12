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
