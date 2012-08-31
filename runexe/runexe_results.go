package main

import (
	"runlib/subprocess"
	"fmt"
	"strconv"
	"os"
	"strings"
)

type Verdict int

const (
	SUCCESS = Verdict(0)
	FAIL = Verdict(1)
	CRASH = Verdict(2)
	TIME_LIMIT_EXCEEDED = Verdict(3)
	MEMORY_LIMIT_EXCEEDED = Verdict(4)
	IDLE = Verdict(5)
	SECURITY_VIOLATION = Verdict(6)
)

func (v Verdict) String() string {
	switch v {
	case SUCCESS:
		return "SUCCESS"
	case FAIL:
		return "FAIL"
	case CRASH:
		return "CRASH"
	case TIME_LIMIT_EXCEEDED:
		return "TIME_LIMIT_EXCEEDED"
	case MEMORY_LIMIT_EXCEEDED:
		return "MEMORY_LIMIT_EXCEEDED"
	case IDLE:
		return "IDLENESS_LIMIT_EXCEEDED"
	case SECURITY_VIOLATION:
		return "SECURITY_VIOLATION"
	}
	return "CRASH"
}

func GetVerdict(r *subprocess.SubprocessResult) Verdict {
	switch {
	case r.SuccessCode == 0:
		return SUCCESS
	case r.SuccessCode & (subprocess.EF_PROCESS_LIMIT_HIT | subprocess.EF_PROCESS_LIMIT_HIT_POST) != 0:
		return SECURITY_VIOLATION
	case r.SuccessCode & (subprocess.EF_INACTIVE | subprocess.EF_TIME_LIMIT_HARD) != 0:
		return IDLE
	case r.SuccessCode & (subprocess.EF_TIME_LIMIT_HIT | subprocess.EF_TIME_LIMIT_HIT_POST) != 0:
		return TIME_LIMIT_EXCEEDED
	case r.SuccessCode & (subprocess.EF_MEMORY_LIMIT_HIT | subprocess.EF_MEMORY_LIMIT_HIT_POST) != 0:
		return MEMORY_LIMIT_EXCEEDED
	default:
		return CRASH
	}
	return CRASH
}

const XML_HEADER = "<?xml version = \"1.1\" encoding = \"UTF-8\"?>"
const XML_RESULTS_START = "<invocationResults>"
const XML_RESULTS_END = "</invocationResults>"

func printTag(tag, content string) {
	fmt.Printf("<%s>%s</%s>\n", tag, content, tag)
}

func xmlTime(t uint64) string {
	return strconv.FormatUint(t / 1000, 10)
}

func PrintResultXml(result *RunResult) {
	fmt.Printf("<invocationResult id=\"%s\">\n", strings.ToLower(result.T.String()))

	printTag("invocationVerdict", result.V.String())
	if result.R != nil {
		printTag("exitCode", strconv.Itoa(int(result.R.ExitCode)))
		printTag("processorUserModeTime", xmlTime(result.R.UserTime))
		printTag("processorKernelModeTime", xmlTime(result.R.KernelTime))
	printTag("passedTime", xmlTime(result.R.WallTime))
	printTag("consumedMemory", strconv.Itoa(int(result.R.PeakMemory)))
	}

	if result.E != nil {
		printTag("comment", result.E.Error())
	}
	fmt.Println("</invocationResult>")
}

func strTime(t uint64) string {
	return strconv.FormatFloat(float64(t) / 1000000, 'f', 2, 64)
}

func strMemory(t uint64) string {
	return strconv.FormatUint(t, 10)
}

func PrintResultText(kernelTime bool, result *RunResult) {
	usuffix := "sec"
	switch result.V {
	case SUCCESS:
		fmt.Println(result.T.String(), "successfully terminated")
		fmt.Println("  exit code:    " + strconv.Itoa(int(result.R.ExitCode)))
	case TIME_LIMIT_EXCEEDED:
		fmt.Println("Time limit exceeded")
		fmt.Println(result.T.String(), "failed to terminate within", strTime(result.S.TimeLimit), "sec")
		usuffix = "of " + strTime(result.S.TimeLimit) + " sec"
	case MEMORY_LIMIT_EXCEEDED:
		fmt.Println("Memory limit exceeded")
		fmt.Println(result.T.String(), "tried to allocate more than", strMemory(result.S.MemoryLimit), "bytes")
	case IDLE:
		fmt.Println("Idleness limit exceeded")
		fmt.Println("Detected", result.T.String(), "idle")
	case SECURITY_VIOLATION:
		fmt.Println("Security violation")
		fmt.Println(result.T.String(), " tried to do some forbidden action")
	case CRASH:
		fmt.Println("Invocation crashed:", result.T.String())
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
	fmt.Println("  time passed: " + strTime(result.R.WallTime) + " sec")
	fmt.Println("  peak memory: " + strMemory(result.R.PeakMemory) + " bytes")
	fmt.Println()
}

func PrintResult(xml, kernelTime bool, result *RunResult) {
	if xml {
		PrintResultXml(result)
	} else {
		PrintResultText(kernelTime, result)
	}
}

type RunResult struct {
	V Verdict
	E error
	S *subprocess.Subprocess
	R *subprocess.SubprocessResult
	T ProcessType
}

func Fail(xml bool, err error) {
	if xml {
		FailXml(err)
	} else {
		FailText(err)
	}
	os.Exit(3)
}

func FailText(err error) {
	fmt.Println("Invocation failed")
	fmt.Println("Comment: ", err)
	fmt.Println()
	fmt.Println("Use \"runexe -h\" to get help information")
}

func FailXml(err error) {
	fmt.Println("<invocationResults>")
	fmt.Println("<invocationResult id=\"program\">")
	fmt.Println("<invocationVerdict>FAIL</invocationVerdict>")
	fmt.Println("<comment>", err, "</comment>")
	fmt.Println("</invocationResult>")
	fmt.Println("</invocationResults>")
}
