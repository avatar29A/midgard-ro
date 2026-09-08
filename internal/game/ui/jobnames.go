package ui

import "github.com/Faultbox/midgard-ro/internal/game/jobs"

func getJobName(jobID uint16) string { return jobs.Name(jobID) }
