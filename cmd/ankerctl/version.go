package main

import "runtime/debug"

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	return versionFromBuildInfo(info)
}
func versionFromBuildInfo(info *debug.BuildInfo) string {
	if info == nil {
		return "dev"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	version := "dev"
	dirty := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			version += "-" + s.Value[:min(12, len(s.Value))]
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if dirty {
		version += "-dirty"
	}
	return version
}
