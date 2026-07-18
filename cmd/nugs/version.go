package main

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
	"strings"
)

// These values are set by release builds with -ldflags. Development builds
// fall back to the module build information embedded by the Go toolchain.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func isVersionRequest(args []string) bool {
	return len(args) == 1 && (args[0] == "version" || args[0] == "--version" || args[0] == "-v")
}

func resolvedBuildIdentity() (resolvedVersion, resolvedCommit, resolvedDate string, dirty bool) {
	resolvedVersion, resolvedCommit, resolvedDate = version, commit, buildDate
	if info, ok := debug.ReadBuildInfo(); ok {
		if resolvedVersion == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			resolvedVersion = info.Main.Version
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if resolvedCommit == "unknown" && setting.Value != "" {
					resolvedCommit = setting.Value
				}
			case "vcs.time":
				if resolvedDate == "unknown" && setting.Value != "" {
					resolvedDate = setting.Value
				}
			case "vcs.modified":
				dirty = strings.EqualFold(setting.Value, "true")
			}
		}
	}
	return resolvedVersion, resolvedCommit, resolvedDate, dirty
}

func printVersion(w io.Writer) {
	resolvedVersion, resolvedCommit, resolvedDate, dirty := resolvedBuildIdentity()
	dirtySuffix := ""
	if dirty {
		dirtySuffix = "+dirty"
	}
	fmt.Fprintf(w, "nugs %s\ncommit: %s%s\nbuilt: %s\ngo: %s\nplatform: %s/%s\n",
		resolvedVersion, resolvedCommit, dirtySuffix, resolvedDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
