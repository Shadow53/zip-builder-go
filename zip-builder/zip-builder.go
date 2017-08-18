package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gitlab.com/Shadow53/zip-builder/build"
	"gitlab.com/Shadow53/zip-builder/config"
)

func main() {
	var destination string
	var configPath string
	config.InitFlags(&destination, &configPath)
	flag.Parse()

	viper.SetDefault("destination", "./build/")
	// All config files must be called "build"...
	if configPath == "" {
		viper.SetConfigName("build")
		viper.AddConfigPath(".")
	} else {
		lastSep := strings.LastIndex(configPath, "/") + 1
		path := configPath[:lastSep]
		file := configPath[lastSep:strings.LastIndex(configPath, ".")]
		viper.SetConfigName(file)
		viper.AddConfigPath(path)
	}

	err := viper.ReadInConfig()

	if err != nil {
		fmt.Printf("Error while reading the configuration file:\n  %v\n", err)
		os.Exit(1)
	}

	if destination != "" {
		viper.Set("destination", destination)
	}

	// Create temporary directory, use this
	dir, tmpErr := ioutil.TempDir("", "zip-builder-")
	defer os.RemoveAll(dir)
	viper.Set("tempdir", dir)

	// This would be the mother of all collisions
	// Anyone specifying a random directory like /tmp/zip-builder-238943
	// should expect a *slight* chance of erroring
	if viper.GetString("tempdir") == dir {
		if tmpErr != nil {
			fmt.Printf("Error while creating a temporary directory:\n  %v\n", tmpErr)
			os.Exit(1)
		} else {

		}
	}

	absDest, err := filepath.Abs(viper.GetString("destination"))
	if err != nil {
		fmt.Printf("Error while converting %v to an absolute path:\n  %v\n", viper.GetString("destination"), err)
		os.Exit(1)
	}
	viper.Set("destination", absDest)

	// Load configuration to memory
	zips, apps, files, err := config.MakeConfig()
	if err != nil {
		fmt.Printf("Error occurred while building configuration:\n  %v\n", err)
		os.Exit(1)
	}
	// Build each zip
	for _, zip := range zips {
		if zip.Name != "" {
			err = build.MakeZip(zip, apps, files)
			if err != nil {
				fmt.Printf("Error while building zip %v:\n  %v\n", zip.Name, err)
			}
		}
	}
}
