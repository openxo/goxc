package config

import (
	"github.com/openxo/goxc/core"
	"log"
	"runtime"
)

func FillBuildSettingsDefaults(bs *BuildSettings) {
	bs.LdFlagsXVars = &map[string]interface{}{"TimeNow": "main.BUILD_DATE", "Version": "main.VERSION"}
}

//TODO fulfil all defaults
func FillSettingsDefaults(settings *Settings) {
	if settings.ResourcesInclude == "" {
		settings.ResourcesInclude = core.RESOURCES_INCLUDE_DEFAULT
	}
	if settings.ResourcesExclude == "" {
		settings.ResourcesExclude = core.RESOURCES_EXCLUDE_DEFAULT
	}
	if settings.PackageVersion == "" {
		settings.PackageVersion = core.PACKAGE_VERSION_DEFAULT
	}
	if settings.BuildSettings == nil {
		bs := BuildSettings{}
		FillBuildSettingsDefaults(&bs)
		settings.BuildSettings = &bs
	}
	if settings.GoRoot == "" {
		log.Printf("Defaulting GoRoot to runtime.GOROOT (%s)", runtime.GOROOT())
		settings.GoRoot = runtime.GOROOT()
	}
}
