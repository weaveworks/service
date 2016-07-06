package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/olekukonko/tablewriter"
	"gopkg.in/yaml.v2"

	"github.com/weaveworks/service/deploy/client"
	"github.com/weaveworks/service/deploy/common"
)

func env(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return def
}

var (
	token   = env("SERVICE_TOKEN", "")
	baseURL = env("BASE_URL", "https://cloud.weave.works")
)

func usage() {
	fmt.Println(`Usage:
		deploy <image>:<version>
		list
		config (<filename>)
		logs <deploy>`)
}

func main() {
	if len(os.Args) <= 1 {
		usage()
		os.Exit(1)
	}

	c := client.New(token, baseURL)

	switch os.Args[1] {
	case "deploy":
		deploy(c, os.Args[2:])
	case "list":
		list(c, os.Args[2:])
	case "config":
		config(c, os.Args[2:])
	case "logs":
		logs(c, os.Args[2:])
	default:
		usage()
	}
}

func deploy(c client.Client, args []string) {
	if len(args) != 1 {
		usage()
		return
	}
	parts := strings.SplitN(args[0], ":", 2)
	if len(parts) < 2 {
		usage()
		return
	}
	deployment := common.Deployment{
		ImageName: parts[0],
		Version:   parts[1],
	}
	if err := c.Deploy(deployment); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func list(c client.Client, args []string) {
	if len(args) != 0 {
		usage()
		return
	}
	status, err := c.GetStatus()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Image", "Version", "State"})
	table.SetBorder(false)
	table.SetColumnSeparator(" ")
	for _, deployment := range status.Deployments {
		table.Append([]string{
			deployment.ID,
			deployment.ImageName,
			deployment.Version,
			deployment.State,
		})
	}
	table.Render()
}

func loadConfig(filename string) (*common.Config, error) {
	extension := filepath.Ext(filename)
	var config common.Config
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if extension == ".yaml" || extension == ".yml" {
		if err := yaml.Unmarshal(buf, &config); err != nil {
			return nil, err
		}
	} else {
		if err := json.NewDecoder(bytes.NewReader(buf)).Decode(&config); err != nil {
			return nil, err
		}
	}
	return &config, nil
}

func config(c client.Client, args []string) {
	if len(args) > 1 {
		usage()
		return
	}

	if len(args) == 1 {
		config, err := loadConfig(args[0])
		if err != nil {
			fmt.Println("Error reading config:", err)
			os.Exit(1)
		}

		if err := c.SetConfig(config); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	} else {
		config, err := c.GetConfig()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		buf, err := yaml.Marshal(config)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		fmt.Println(string(buf))
	}
}

func logs(c client.Client, args []string) {
	if len(args) != 1 {
		usage()
		return
	}

	output, err := c.GetLogs(args[0])
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Println(string(output))
}
