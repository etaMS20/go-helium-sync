package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"text/tabwriter"
)

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: hesync <command>")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Commands:")

	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "  setup [--url <remote-url>]\tconnect this device to the sync repo (guided without --url)")
	fmt.Fprintln(w, "  sync\tsync Helium bookmarks with the repo")
	fmt.Fprintln(w, "  version, --version\tshow the current version")
	fmt.Fprintln(w, "  help, -h, --help\tshow this help")
	w.Flush()
	fmt.Fprintln(out)
}

var version = "dev" // overridden via -ldflags "-X main.version=1.2.3"

func getVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

// parse parses a subcommand's flags. ExitOnError: -h/--help exits 0, a bad flag exits 2.
// Positional arguments aren't used by any subcommand, so they're an error too.
func parse(fs *flag.FlagSet, usage string, args []string) {
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s\n", usage)
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if fs.NArg() > 0 {
		fmt.Fprintf(fs.Output(), "unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		os.Exit(2)
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "setup":
		fs := flag.NewFlagSet("setup", flag.ExitOnError)
		url := fs.String("url", "", "remote repo URL (asked interactively if omitted)")
		parse(fs, "hesync setup [--url <remote-url>]", args)
		err = setup(*url)
	case "sync":
		parse(flag.NewFlagSet("sync", flag.ExitOnError), "hesync sync", args)
		err = runSync()
	case "help", "-h", "--help":
		printUsage(os.Stdout)
	case "version", "--version":
		fmt.Printf("hesync %s\n", getVersion())
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		printUsage(os.Stderr)
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}
