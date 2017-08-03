package main

import (
	"io/ioutil"
	"lib"
	"log"
	//"os"

	"github.com/spf13/viper"
)

func main() {
	dir, err := ioutil.TempDir("", "example")
	//defer os.RemoveAll(dir)

	viper.SetDefault("tempdir", dir)
	viper.SetDefault("destination", ".")
	viper.SetConfigName("build")
	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		log.Panicf("Fatal error on config file: %s \n", err)
	}
	zips, apps, files := lib.MakeConfig()
	/*log.Println("Apps:")
	log.Println(apps)
	log.Println("Files:")
	log.Println(files)
	log.Println("Zips:")
	log.Println(zips)*/
    
    for _, zip := range zips {
        MakeZip(zip, apps, files)
    }
}
