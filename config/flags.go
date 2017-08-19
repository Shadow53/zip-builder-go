package config

import "flag"

func InitFlags(destination *string, configPath *string, debug *bool) {
	flag.StringVar(destination, "destination", "", "The folder to place the generated zip(s) into")
	flag.StringVar(configPath, "config", "", "Path to configuration file to use")
	flag.BoolVar(debug, "debug", false, "Enable debugging output")
}
