package main

import (
    "build"
    "config"
	"io/ioutil"
	"lib"
	"log"
	"os"
    "path/filepath"
    
	"github.com/spf13/viper"
)

func main() {
    // Create temporary directory, set as default
	dir, err := ioutil.TempDir("", "zip-builder-")
	defer os.RemoveAll(dir)
	viper.SetDefault("tempdir", dir)
    // Get current working directory, set as default destination
    ex, err := os.Executable()
    lib.ExitIfError(err)
    cwd := filepath.Dir(ex)
    viper.SetDefault("destination", cwd)
    // All config files must be called "build"...
	viper.SetConfigName("build")
    // ... and read from the current directory
	viper.AddConfigPath(cwd)
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
