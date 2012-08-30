package main

import (
	"runlib/subprocess"
	"fmt"
	"strconv"
	"bytes"
	"io"
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

func XmlResult(r *subprocess.SubprocessResult, c string) []byte {
	var result bytes.Buffer
	v := GetVerdict(r)
	fmt.Fprintln(&result, "<invocationResult>")
	fmt.Fprintln(&result, "    <invocationVerdict>" + v.String() + "</invocationVerdict>")
	fmt.Fprintln(&result, "    <exitCode>" +
			strconv.Itoa(int(r.ExitCode)) +
			"</exitCode>")
	fmt.Fprintln(&result, "    <processorUserModeTime>" +
			strconv.Itoa(int(r.UserTime / 1000)) +
			"</processorUserModeTime>")
	fmt.Fprintln(&result, "    <processorKernelModeTime>" +
			strconv.Itoa(int(r.KernelTime / 1000)) +
			"</processorKernelModeTime>")
	fmt.Fprintln(&result, "    <passedTime>" +
			strconv.Itoa(int(r.WallTime / 1000)) +
			"</passedTime>")
	fmt.Fprintln(&result, "    <consumedMemory>" +
			strconv.Itoa(int(r.PeakMemory)) +
			"</consumedMemory>");
	fmt.Fprintln(&result, "    <comment>" + c +
			"</comment>");
	fmt.Fprintln(&result, "</invocationResult>")

	return result.Bytes()
}

func strTime(t uint64) string {
	return strconv.FormatFloat(float64(t) / 1000000, 'f', 2, 64)
}

func strMemory(t uint64) string {
	return strconv.FormatUint(t, 10)
}

func PrintResult(out io.Writer, s *subprocess.Subprocess, r *subprocess.SubprocessResult, c string, kernelTime bool) {
	v := GetVerdict(r)

	switch v {
	case SUCCESS:
		fmt.Fprintln(out, c + " successfully terminated")
		fmt.Fprintln(out, "  exit code:    " + strconv.Itoa(int(r.ExitCode)))
	case TIME_LIMIT_EXCEEDED:
		fmt.Fprintln(out, "Time limit exceeded")
		fmt.Fprintln(out, c + " failed to terminate within " + strTime(s.TimeLimit) + " sec")
	case MEMORY_LIMIT_EXCEEDED:
		fmt.Fprintln(out, "Memory limit exceeded")
		fmt.Fprintln(out, c + " tried to allocate more than " + strMemory(s.MemoryLimit) + " bytes")
	case IDLE:
		fmt.Fprintln(out, "Idleness limit exceeded")
		fmt.Fprintln(out, "Detected " + c + " idle")
	case SECURITY_VIOLATION:
		fmt.Fprintln(out, "Security violation")
		fmt.Fprintln(out, c + " tried to do some forbidden action")
	}

	var usuffix string
	if v == TIME_LIMIT_EXCEEDED {
		usuffix = "of " + strTime(s.TimeLimit) + " sec"
	} else {
		usuffix = "sec"
	}
	utime := strTime(r.UserTime) + " " + usuffix
	if kernelTime {
		fmt.Fprintln(out, "  time consumed:")
		fmt.Fprintln(out, "    user mode:   " + utime)
		fmt.Fprintln(out, "    kernel mode: " + strTime(r.KernelTime) + " sec")
	} else {
		fmt.Fprintln(out, "  time consumed: " + utime)
	}
	fmt.Fprintln(out, "  time passed: " + strTime(r.WallTime) + " sec")
	fmt.Fprintln(out, "  peak memory: " + strMemory(r.PeakMemory) + " bytes")
	fmt.Fprintln(out)
}

func Crash(out io.Writer, comment string, e error) {
	fmt.Fprintln(out, "Invocation crashed")
	fmt.Fprintln(out, "Comment: " + comment)
	if e != nil {
		fmt.Fprintln(out, "Error:", e)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Use \"runexe -h\" to get help information");
}
