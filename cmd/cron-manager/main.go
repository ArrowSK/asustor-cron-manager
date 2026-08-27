package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ArrowSK/asustor-cron-manager/internal/auth"
	cronpkg "github.com/ArrowSK/asustor-cron-manager/internal/cron"
	"github.com/ArrowSK/asustor-cron-manager/internal/history"
	"github.com/ArrowSK/asustor-cron-manager/internal/server"
	updatepkg "github.com/ArrowSK/asustor-cron-manager/internal/update"
)

var version = "dev"

const defaultInstallDir = "/usr/local/AppCentral/CronManager"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "exec":
			execMode(os.Args[2:])
			return
		case "unmanage-all":
			unmanageMode()
			return
		case "version":
			fmt.Println(version)
			return
		}
	}

	fs := flag.NewFlagSet("cron-manager", flag.ExitOnError)
	listen := fs.String("listen", envOr("ACM_LISTEN", "0.0.0.0:18367"), "HTTP listen address")
	data := fs.String("data", envOr("ACM_DATA", filepath.Join(defaultInstallDir, "data")), "persistent data directory")
	_ = fs.Parse(os.Args[1:])

	if os.Geteuid() != 0 {
		log.Fatal("Cron Manager must run as root so it can manage the root crontab")
	}
	if err := os.MkdirAll(*data, 0o700); err != nil {
		log.Fatal(err)
	}
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)
	runnerPath := envOr("ACM_RUNNER", filepath.Join(defaultInstallDir, "cron-manager"))
	cs := cronpkg.NewStore(filepath.Join(*data, "backups"), runnerPath)
	a := auth.New(filepath.Join(*data, "auth.json"))
	hs := history.New(filepath.Join(*data, "history.json"))
	u := updatepkg.New(version, runnerPath)
	s := server.New(cs, a, hs, u, version, logger)
	if err := server.ListenAndServe(*listen, s.Handler(), logger); err != nil {
		log.Fatal(err)
	}
}

func execMode(args []string) {
	if os.Geteuid() != 0 {
		log.Fatal("exec mode requires root")
	}
	if len(args) != 1 {
		log.Fatal("usage: cron-manager exec <job-id>")
	}
	data := envOr("ACM_DATA", filepath.Join(defaultInstallDir, "data"))
	cs := cronpkg.NewStore(filepath.Join(data, "backups"), filepath.Join(defaultInstallDir, "cron-manager"))
	job, err := cs.ResolveJob(args[0])
	if err != nil {
		log.Fatal(err)
	}
	if !job.Managed || !job.Enabled {
		log.Fatal("managed job is disabled or unavailable")
	}
	rec := server.Execute(job.ID, job.Command, "cron", cronpkg.NewID(), 0)
	hs := history.New(filepath.Join(data, "history.json"))
	if err := hs.Add(rec); err != nil {
		log.Printf("history: %v", err)
	}
	os.Exit(rec.ExitCode)
}

func unmanageMode() {
	if os.Geteuid() != 0 {
		log.Fatal("unmanage-all requires root")
	}
	data := envOr("ACM_DATA", filepath.Join(defaultInstallDir, "data"))
	cs := cronpkg.NewStore(filepath.Join(data, "backups"), filepath.Join(defaultInstallDir, "cron-manager"))
	if err := cs.UnmanageAll(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Cron Manager jobs converted back to plain crontab entries")
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

var _ = time.Second
