package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/contester/runlib/subprocess"
)

type verdict int

const (
	verdictSuccess             = verdict(0)
	verdictFail                = verdict(1)
	verdictCrash               = verdict(2)
	verdictTimeLimitExceeded   = verdict(3)
	verdictMemoryLimitExceeded = verdict(4)
	verdictIdle                = verdict(5)
	verdictSecurityViolation   = verdict(6)
)

func (v verdict) String() string {
	switch v {
	case verdictSuccess:
		return "SUCCEEDED"
	case verdictFail:
		return "FAILED"
	case verdictCrash:
		return "CRASHED"
	case verdictTimeLimitExceeded:
		return "TIME_LIMIT_EXCEEDED"
	case verdictMemoryLimitExceeded:
		return "MEMORY_LIMIT_EXCEEDED"
	case verdictIdle:
		return "IDLENESS_LIMIT_EXCEEDED"
	case verdictSecurityViolation:
		return "SECURITY_VIOLATION"
	}
	return "FAILED"
}

func getVerdict(r *subprocess.SubprocessResult) verdict {
	switch {
	case r.SuccessCode == 0:
		return verdictSuccess
	case r.SuccessCode&(subprocess.EF_PROCESS_LIMIT_HIT|subprocess.EF_PROCESS_LIMIT_HIT_POST) != 0:
		return verdictSecurityViolation
	case r.SuccessCode&(subprocess.EF_INACTIVE|subprocess.EF_TIME_LIMIT_HARD) != 0:
		return verdictIdle
	case r.SuccessCode&(subprocess.EF_TIME_LIMIT_HIT|subprocess.EF_TIME_LIMIT_HIT_POST) != 0:
		return verdictTimeLimitExceeded
	case r.SuccessCode&(subprocess.EF_MEMORY_LIMIT_HIT|subprocess.EF_MEMORY_LIMIT_HIT_POST) != 0:
		return verdictMemoryLimitExceeded
	default:
		return verdictCrash
	}
}

type invocationSuccess struct {
	XMLName    xml.Name `xml:"invocationResult"`
	ID         string   `xml:"id,attr"`
	Verdict    string   `xml:"invocationVerdict"`
	ExitCode   int      `xml:"exitCode"`
	UserTime   int      `xml:"processorUserModeTime"`
	KernelTime int      `xml:"processorKernelModeTime"`
	WallTime   int      `xml:"passedTime"`
	Memory     int      `xml:"consumedMemory"`
}

type invocationError struct {
	XMLName xml.Name `xml:"invocationResult"`
	ID      string   `xml:"id,attr"`
	Error   string   `xml:"comment,omitempty"`
}

type invocationResults struct {
	XMLName xml.Name `xml:"invocationResults"`
	Result  []interface{}
}

const xmlHeaderText = "<?xml version=\"1.1\" encoding=\"UTF-8\"?>"

func PrintResultsXml(results []*RunResult) {
	r := invocationResults{}
	for _, v := range results {
		if v != nil {
			r.Result = append(r.Result, convertXml(v))
		}
	}
	d, err := xml.MarshalIndent(&r, "", "  ")
	if err != nil {
		return
	}
	fmt.Println(string(d))
}

func convertXml(result *RunResult) interface{} {
	if result.R != nil {
		return invocationSuccess{
			ID:         strings.ToLower(result.T.String()),
			Verdict:    result.V.String(),
			ExitCode:   int(result.R.ExitCode),
			UserTime:   int(result.R.UserTime.Nanoseconds() / 1000000),
			KernelTime: int(result.R.KernelTime.Nanoseconds() / 1000000),
			WallTime:   int(result.R.WallTime.Nanoseconds() / 1000000),
			Memory:     int(result.R.PeakMemory),
		}
	}

	if result.E != nil {
		return invocationError{
			ID:    strings.ToLower(result.T.String()),
			Error: result.E.Error(),
		}
	}
	return nil
}

func strTime(t time.Duration) string {
	return strconv.FormatFloat(t.Seconds(), 'f', 2, 64)
}

func strMemory(t uint64) string {
	return strconv.FormatUint(t, 10)
}

func PrintResultText(kernelTime bool, result *RunResult, pipeRecords []subprocess.PipeRecordEntry) {
	usuffix := "sec"
	switch result.V {
	case verdictSuccess:
		fmt.Println(result.T.String(), "successfully terminated")
		fmt.Println("  exit code:    " + strconv.Itoa(int(result.R.ExitCode)))
	case verdictTimeLimitExceeded:
		fmt.Println("Time limit exceeded")
		fmt.Println(result.T.String(), "failed to terminate within", strTime(result.S.TimeLimit), "sec")
		usuffix = "of " + strTime(result.S.TimeLimit) + " sec"
	case verdictMemoryLimitExceeded:
		fmt.Println("Memory limit exceeded")
		fmt.Println(result.T.String(), "tried to allocate more than", strMemory(result.S.MemoryLimit), "bytes")
	case verdictIdle:
		fmt.Println("Idleness limit exceeded")
		fmt.Println("Detected", result.T.String(), "idle")
	case verdictSecurityViolation:
		fmt.Println("Security violation")
		fmt.Println(result.T.String(), " tried to do some forbidden action")
	case verdictCrash:
		fmt.Println("Invocation crashed:", result.T.String())
		fmt.Println("Comment:", result.E)
		fmt.Println()
		return
	case verdictFail:
		fmt.Println("Invocation failed:", result.T.String())
		fmt.Println("Comment:", result.E)
		fmt.Println()
		return
	}

	utime := strTime(result.R.UserTime) + " " + usuffix
	if kernelTime {
		fmt.Println("  time consumed:")
		fmt.Println("    user mode:   " + utime)
		fmt.Println("    kernel mode: " + strTime(result.R.KernelTime) + " sec")
	} else {
		fmt.Println("  time consumed: " + utime)
	}
	fmt.Println("  time passed:  " + strTime(result.R.WallTime) + " sec")
	fmt.Println("  peak memory:  " + strMemory(result.R.PeakMemory) + " bytes")
	fmt.Println()

	for _, v := range pipeRecords {
		fmt.Printf("%+v\n", v)
	}
}

type RunResult struct {
	V verdict
	E error
	S *subprocess.Subprocess
	R *subprocess.SubprocessResult
	T processType
}

var failLog = FailText

func Fail(err error, state string) {
	failLog(err, state)
	os.Exit(3)
}

func FailText(err error, state string) {
	fmt.Println("Invocation failed")
	fmt.Println("Comment: (", state, ") ", err)
	fmt.Println()
	fmt.Println("Use \"runexe -h\" to get help information")
}

func FailXml(err error, state string) {
	r := invocationResults{
		Result: []interface{}{
			invocationError{
				ID:    "program",
				Error: "(" + state + ") " + err.Error(),
			},
		},
	}
	d, err := xml.MarshalIndent(&r, "", "  ")
	if err != nil {
		return
	}
	fmt.Println(string(d))
}
