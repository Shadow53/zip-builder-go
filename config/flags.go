package config

import "flag"

func InitFlags(destination *string, configPath *string) {
	flag.StringVar(destination, "destination", "", "The folder to place the generated zip(s) into")
	flag.StringVar(configPath, "config", "", "Path to configuration file to use")
}
