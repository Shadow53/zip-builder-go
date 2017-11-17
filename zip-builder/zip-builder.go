package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"gitlab.com/Shadow53/zip-builder/build"
	"gitlab.com/Shadow53/zip-builder/config"
	"gitlab.com/Shadow53/zip-builder/lib"
)

func main() {
	var destination string
	var configPath string
	var verbose bool
	var debug bool
	config.InitFlags(&destination, &configPath, &verbose, &debug)
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

	if debug {
		viper.Set("debug", true)
	}

	if verbose {
		viper.Set("verbose", true)
	}

	// Create temporary directory, use this
	dir, tmpErr := ioutil.TempDir("", "zip-builder-")
	//defer os.RemoveAll(dir)
	viper.Set("tempdir", dir)

	if tmpErr != nil {
		fmt.Printf("Error while creating a temporary directory:\n  %v\n", tmpErr)
		os.Exit(1)
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
	var wg sync.WaitGroup
	ch := make(chan error)
	for _, zip := range zips {
		wg.Add(1)
		go func(zip lib.ZipInfo, apps *lib.Apps, files *lib.Files, wg *sync.WaitGroup, ch chan error) {
			defer wg.Done()
			if zip.Name != "" {
				build.MakeZip(&zip, apps, files, ch)
			}
		}(zip, apps, files, &wg, ch)
	}

	var errs []error
	go func(ch *chan error, errs *[]error) {
		for err := range *ch {
			fmt.Println(err)
			*errs = append(*errs, err)
		}
	}(&ch, &errs)

	wg.Wait()
	close(ch)

	for _, err := range errs {
		fmt.Printf("\n%v\n", err)
	}
}
