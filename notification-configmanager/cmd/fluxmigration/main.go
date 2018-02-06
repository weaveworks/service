// This script for migration flux configs to notification receivers

// run on dev:
// go run main.go -database "/usr/home/go/src/github.com/weaveworks/service-conf/infra/database" -env dev

// run on prod:
// go run main.go -database "/usr/home/go/src/github.com/weaveworks/service-conf/infra/database" -env prod

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"

	"github.com/google/uuid"
)

type fluxConfig struct {
	Settings fluxSettings `json:"settings"`
}

type fluxSettings struct {
	Slack slackSettings `json:"slack"`
}

type slackSettings struct {
	HookURL string `json:"hookURL"`
}

func main() {
	var dbshell, environment, ids string
	var dryrun bool
	flag.StringVar(&dbshell, "database", "", "Path to service-conf/infra/database script")
	flag.StringVar(&environment, "env", "dev", "Where to run script: dev/prod")
	flag.BoolVar(&dryrun, "dryrun", true, "only print out statments if true; execute if false")
	flag.StringVar(&ids, "ids", "ids.txt", "File to save all instance IDs with not empty flux slack config")
	flag.Parse()
	log.SetFlags(0)

	if dbshell == "" {
		log.Fatal("specify file path for infra/database script")
	}

	exportCmd := exec.Command(dbshell, "shell", environment, "fluxy_vpc")
	var selectStr bytes.Buffer
	selectStr.WriteString("SELECT instance, config FROM config where config not like '%hookURL\":\"\"%';")
	exportCmd.Stdin = &selectStr

	exportOut, err := exportCmd.CombinedOutput()
	if err != nil {
		log.Fatalf("cannot export flux configs, output: %s; error: %s", exportOut, err)
	}

	var buf, bufIDs bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(exportOut))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "hookURL") {
			ss := strings.Split(line, "|")

			instanceID := strings.TrimSpace(ss[0])
			config := ss[1]
			dec := json.NewDecoder(strings.NewReader(config))

			var flxCfg fluxConfig
			if err := dec.Decode(&flxCfg); err != nil {
				log.Fatalf("cannot decode config %s, error %s", config, err)
			}

			url := flxCfg.Settings.Slack.HookURL
			if strings.HasPrefix(url, "https://hooks.slack.com/services/") {
				rcvID := uuid.New()
				insertReceivers := fmt.Sprintf("insert into receivers (receiver_id, instance_id, receiver_type, address_data) values ('%s', '%s', 'slack', '\"%s\"'); ", rcvID, instanceID, url)
				insertEventTypes := fmt.Sprintf("insert into receiver_event_types (receiver_id, event_type) select '%s', name from event_types where 'slack' = any (default_receiver_types); ", rcvID)
				insertStr := insertReceivers + insertEventTypes
				log.Printf("instanceID = %s; URL %q", instanceID, url)
				buf.WriteString(insertStr)
				bufIDs.WriteString(instanceID + "\n")
			} else {
				log.Printf("instanceID = %s; URL %q doesn't have prefix 'https://hooks.slack.com/services/', skip and continue...", instanceID, url)
			}
		}
	}

	if !dryrun {
		insertCmd := exec.Command(dbshell, "shell", environment, "notification_configs_vpc", "notifications")
		insertCmd.Stdin = &buf
		insertOut, err := insertCmd.CombinedOutput()
		if err != nil {
			log.Fatalf("cannot insert receivers, output: %s; error: %s", insertOut, err)
		}
		log.Printf("done; ouptut:\n%s", insertOut)
	} else {
		log.Println("This is a dry run, the above statements were not executed.")
	}

	if err := ioutil.WriteFile(ids, bufIDs.Bytes(), 0644); err != nil {
		log.Fatalf("cannot write to file %s list of instance IDs, error: %s", ids, err)
	}
}
