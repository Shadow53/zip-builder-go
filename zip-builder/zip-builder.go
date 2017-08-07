package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/viper"
	"gitlab.com/Shadow53/zip-builder/build"
	"gitlab.com/Shadow53/zip-builder/config"
)

func main() {
	// Create temporary directory, set as default
	dir, err := ioutil.TempDir("", "zip-builder-")
	defer os.RemoveAll(dir)
	viper.SetDefault("tempdir", dir)
	// All config files must be called "build"...
	viper.SetConfigName("build")
	viper.AddConfigPath(".")
	err = viper.ReadInConfig()

	if err != nil {
		log.Panicf("Fatal error on config file: %s \n", err)
	}

	// Load configuration to memory
	zips, apps, files := config.MakeConfig()

	// Build each zip
	for _, zip := range zips {
		if zip.Name != "" {
			build.MakeZip(zip, apps, files)
		}
	}
}
