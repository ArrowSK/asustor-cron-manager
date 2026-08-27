package cron

import "time"

type Job struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Schedule string      `json:"schedule"`
	Command  string      `json:"command"`
	Enabled  bool        `json:"enabled"`
	Managed  bool        `json:"managed"`
	Human    string      `json:"human"`
	LastRun  *RunSummary `json:"lastRun,omitempty"`
	Running  bool        `json:"running"`
}

type RunSummary struct {
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished"`
	ExitCode int       `json:"exitCode"`
	Source   string    `json:"source"`
}

type ManagedBlock struct {
	ID       string
	Name     string
	Schedule string
	Command  string
	Enabled  bool
	Start    int
	End      int
}

type RawJob struct {
	ID       string
	Schedule string
	Command  string
	Line     string
	Index    int
}
