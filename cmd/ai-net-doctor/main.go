// Command ai-net-doctor diagnoses whether the current network can reach Codex
// and Claude Code, and localizes the first broken layer. The CLI computes the
// verdict; any wrapper (skill, click entry) must only display it.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/wxggzz/ai-net-doctor/internal/model"
	"github.com/wxggzz/ai-net-doctor/internal/report"
	"github.com/wxggzz/ai-net-doctor/internal/runner"
	"github.com/wxggzz/ai-net-doctor/internal/verdict"
)

func main() {
	var (
		targetFlag = flag.String("target", "all", "codex | claude | all")
		jsonOut    = flag.Bool("json", false, "machine-readable JSON output")
		verbose    = flag.Bool("verbose", false, "verbose per-layer waterfall")
		budgetSec  = flag.Int("budget", 15, "total time budget in seconds")
		direct     = flag.Bool("direct", false, "force direct connection (ignore proxy)")
		proxyMode  = flag.String("proxy", "", "force proxy path: env | system")
		showVer    = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Println("ai-net-doctor " + model.Version)
		return
	}

	order := parseTargets(*targetFlag)
	if order == nil {
		fmt.Fprintln(os.Stderr, "invalid --target: use codex | claude | all")
		os.Exit(64)
	}

	forceMode, err := resolveMode(*direct, *proxyMode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(64)
	}

	if *budgetSec <= 0 {
		fmt.Fprintln(os.Stderr, "invalid --budget: must be > 0")
		os.Exit(64)
	}

	rep := runner.Run(context.Background(), runner.Options{
		Targets:   order,
		Budget:    time.Duration(*budgetSec) * time.Second,
		ForceMode: forceMode,
	})

	if *jsonOut {
		out, err := report.JSON(rep)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(out)
	} else {
		fmt.Print(report.Text(rep, order, *verbose))
	}

	os.Exit(verdict.ExitCode(overallVerdict(rep, order)))
}

func parseTargets(t string) []string {
	switch t {
	case "all", "":
		return []string{"codex", "claude"}
	case "codex":
		return []string{"codex"}
	case "claude":
		return []string{"claude"}
	default:
		return nil
	}
}

func resolveMode(direct bool, proxyMode string) (string, error) {
	if direct && proxyMode != "" {
		return "", fmt.Errorf("cannot combine --direct with --proxy")
	}
	if direct {
		return "direct", nil
	}
	switch proxyMode {
	case "":
		return "", nil
	case "env", "system":
		return proxyMode, nil
	default:
		return "", fmt.Errorf("invalid --proxy: use env | system")
	}
}

func overallVerdict(rep model.Report, order []string) model.Verdict {
	var vs []model.Verdict
	for _, name := range order {
		if res, ok := rep.Targets[name]; ok {
			vs = append(vs, res.Verdict)
		}
	}
	return verdict.WorstVerdict(vs...)
}
