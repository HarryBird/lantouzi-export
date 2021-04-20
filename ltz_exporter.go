package main

import (
	"log"
	"os"
	"time"

	"github.com/HarryBird/cdp"
	"github.com/HarryBird/lantouzi-export/download"
	"github.com/HarryBird/lantouzi-export/export"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logger *log.Logger
)

type target struct {
	Url    string
	Name   string
	Screen bool
	Parse  bool
	Column int
}

type config struct {
	Cookies []map[string]interface{}
	Targets []target
}

func initConfig() config {
	viper.AddConfigPath(".")
	viper.SetConfigFile("./config.yaml")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Panicf("%s %s %+v", "[PANIC] ", "Find Config File Fail -> ", err)
		} else {
			logger.Panicf("%s %s %+v", "[PANIC] ", "Load Config File Fail ->", err)
		}
	}

	var conf config

	if err := viper.Unmarshal(&conf); err != nil {
		logger.Panicf("%s %s %+v", "[PANIC] ", "Parse Config File Fail ->", err)
	}

	if len(conf.Cookies) > 0 {
		for _, cookie := range conf.Cookies {
			if _, ok := cookie["ExpireWithIn"]; ok {
				if exp, ok := cookie["ExpireWithIn"].(int); ok {
					cookie["Expires"] = cdp.GetCookieExpireFromInt(exp)
				}
			}
		}
	}

	return conf
}

func runDownload(cmd *cobra.Command, args []string) {
	config := initConfig()

	if len(config.Cookies) == 0 {
		logger.Panicf("%s %s", "[PANIC] ", "Empty Cookie Setting")
	}

	downloader := download.New(
		download.WithCookies(config.Cookies),
	)

	if err := downloader.Run(); err != nil {
		logger.Panicf("%s %s %+v", "[ERROR] ", "Downloader Run Fail", err)
	}

}

func runExport(cmd *cobra.Command, args []string) {
	config := initConfig()

	if len(config.Cookies) == 0 {
		logger.Panicf("%s %s", "[PANIC] ", "Empty Cookie Setting")
	}

	if len(config.Targets) == 0 {
		logger.Panicf("%s %s", "[PANIC] ", "Empty Target Setting")
	}

	for _, target := range config.Targets {
		if target.Url == "" || target.Name == "" {
			logger.Printf("%s %s", "[WARN] ", "Invalid Target, Ignore...")
			continue
		}

		if target.Column == 0 {
			logger.Printf("%s %s", "[WARN] ", "Invalid Target, Ignore...")
			continue
		}

		exporter := export.New(
			export.WithCookies(config.Cookies),
			export.WithUrl(target.Url),
			export.WithName(target.Name),
			export.WithScreen(target.Screen),
			export.WithParse(target.Parse),
			export.WithColumn(target.Column),
		)

		if err := exporter.Run(); err != nil {
			logger.Panicf("%s %s %+v", "[ERROR] ", "Exporter Run Fail", err)
		}

		time.Sleep(1 * time.Second)
	}
}

func main() {
	root := &cobra.Command{
		Use:   "ltz",
		Short: "lantouzi.com export tools",
	}

	export := &cobra.Command{
		Use:   "export",
		Short: "Export Lantouzi.com Account's Records",
		Run:   runExport,
	}

	download := &cobra.Command{
		Use:   "download",
		Short: "Download Lantouzi.com Account's Agrements",
		Run:   runDownload,
	}

	root.AddCommand(export, download)
	root.Execute()
}

func init() {
	logger = log.New(os.Stdout, "<MAIN> ", log.LstdFlags|log.Lshortfile|log.Lmsgprefix)

}
